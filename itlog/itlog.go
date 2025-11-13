/*
itlog is a performant, zero-allocation, logger (~800 SLoC) inspired by ZeroLog (~16k SLoC).
FWIW, this code was written AI-free :)

Design Decisions:

 1. Format

    .......Header.....|.Body..\n
    time|level|message|context\n

    The format is split into the Header and Body. The header is a fixed-width
    chunk, holding fields that are present in all logs. It is split into
    sub-components (1) time, (2) log level, (3) and message, all separated by
    the ComponentDelimiter. The body is a variable-width chunk, holding
    dynamically appended fields. It consists of key=value pairs separated by
    ComponentDelimiter and delimited by KeyValDelimiter. Keys with
    multiple values are appended as individual pairs len(values) times in the
    context, sharing the same key.

    A key characteristic of this design is that it simplifies escaping. There
    are only two special characters to escape.

    - Newline (delimits each log event)
    - Null byte (An unescaped null byte within the log is an implementation error)

 2. No colored output
    At first, I implemented colored output. In practice however, the colors are
    not all that useful when the context gets large and your terminal screen is
    filled with text that hard wrap, breaking visual alignment of the logs. It's
    better to save the logs in a file and explore them in your editor. Another
    benefit is that this simplifies the implementation further.

    TODO: Add a log parser.
*/
package itlog

import (
	"errors"
	"io"
	"os"
	"reflect"
	"strconv"
	"sync"
	"time"
	"unsafe"

	"golang_snacks/invariant"
)

const (
	DefaultLoggerBufferCapacity = DefaultEventBufferCapacity / 2
	DefaultEventBufferCapacity  = HeaderCapacity + 1 + ContextCapacity + len("\n")
	HeaderCapacity              = TimestampCapacity + 1 + LevelCapacity + 1 + MessageCapacity

	TimestampCapacity = len(time.RFC3339) - len("07:00") // hardcoded to always be in UTC
	LevelCapacity     = LevelMaxWordLength
	// You can get a 10ns/op improvement if you reduce this down to 50
	// charaters but it's not worth it for such a short message window.
	MessageCapacity = 80
	// This should cover most cases. Note that the buffers can still grow
	// when the need arises, in which case, they don't get returned to the
	// pool but are instead left for the garbage collector.
	ContextCapacity = 300

	LevelMaxWordLength = 3
	LevelDebug         = -100
	LevelInfo          = 0
	LevelWarn          = 100
	LevelError         = 200
	LevelDisabled      = 500

	// Note: This implementation assumes that these separators are
	// characters and thus have a length of 1.
	ComponentDelimiter   = '|'
	KeyValDelimiter      = '='
	Quote                = '"'
	EmptyIndicatorString = "__EMPTY__"
)

var (
	EmptyIndicatorBytes = []byte(EmptyIndicatorString)
	TickCallback        = func() time.Time {
		return time.Now().UTC()
	}
)

// === Encoding ===
//
// - newline   (0x0A)    -> `\n`
// - nullbyte  (0x00)    -> `\0`
//
// Every null byte in the log is escaped, ensuring that any
// unescaped null byte appearing in the logs can be safely interpreted as data
// corruption.
//
// Notes:
// Other non-readable characters remain unchanged.
// UTF is not handled.
//
// The resulting encoding is **lossy** — it cannot be decoded back to the original data.
func appendEscaped(dst, src []byte) []byte {
	invariant.Always(dst != nil, "appendEscaped must receive a non-nil slice pointer")
	invariant.Always(src != nil, "appendEscaped caller handles empty string argument")

	before := len(dst)
	defer func() {
		after := len(dst)
		invariant.Sometimes(after-before == len(src), "String to encode was appended as is")
		invariant.Sometimes(after-before > len(src), "String to encode contained escaped characters")
	}()
	for _, ch := range src {
		switch ch {
		case '\n':
			invariant.Sometimes(true, "String to encode contains raw newline")
			dst = append(dst, '\\', 'n')
		case 0:
			invariant.Sometimes(true, "String to encode contains raw null byte")
			dst = append(dst, '\\', '0')
		default:
			dst = append(dst, ch)
		}
	}
	return dst
}

func ValidateKey(key []byte) error {
	if len(key) == 0 {
		return errors.New("Key is not empty")
	}
	period := 0
	underscore := 0
	for _, ch := range key {
		switch {
		case 'a' <= ch && ch <= 'z':
		case 'A' <= ch && ch <= 'Z':
		case '0' <= ch && ch <= '9':
		case ch == '.':
			period++
		case ch == '_':
			underscore++
		default:
			return errors.New("Log context key only contains alphanumeric, periods, and underscores")
		}
	}
	if period == len(key) {
		return errors.New("Log context does not only contain periods")
	} else if underscore == len(key) {
		return errors.New("Log context does not only contain underscores")
	} else if period+underscore == len(key) {
		return errors.New("Log context does not only contain periods and underscores")
	}
	return nil
}

// === PRIMITIVES ===

// Primitive
// data appends a key-value pair to the _Event context.
func (event *_Event) data(key, val []byte) *_Event {
	if event == nil {
		invariant.Unreachable("event.data callers don't propagate nil events")
		return nil
	}
	if key == nil {
		key = EmptyIndicatorBytes
	}
	if val == nil {
		val = EmptyIndicatorBytes
	}
	invariant.AlwaysNil(ValidateKey(key), "Log context key is valid")
	event.Buffer = append(event.Buffer, key...)
	event.Buffer = append(event.Buffer, KeyValDelimiter)
	event.Buffer = append(event.Buffer, val...)
	event.Buffer = append(event.Buffer, ComponentDelimiter)
	return event
}

// Primitive
// String context values are wrapped in quotes. Quote characters inside the
// value are left unescaped. Parsing requires at least two quote delimiters;
// everything between them is taken literally.
func (event *_Event) Str(key, val string) *_Event {
	if event == nil {
		invariant.Sometimes(true, "event.Str event is nil")
		return nil
	}
	if key == "" {
		key = EmptyIndicatorString
	}
	if val == "" {
		val = EmptyIndicatorString
	}
	invariant.AlwaysNil(ValidateKey(stringToBytesUnsafe(key)), "Log context key is valid")
	invariant.Sometimes(true, "Add context to event")

	event.Buffer = appendEscaped(event.Buffer, stringToBytesUnsafe(key))
	event.Buffer = append(event.Buffer, KeyValDelimiter)
	event.Buffer = append(event.Buffer, Quote)
	event.Buffer = appendEscaped(event.Buffer, stringToBytesUnsafe(val))
	event.Buffer = append(event.Buffer, Quote)
	event.Buffer = append(event.Buffer, ComponentDelimiter)

	return event
}

// Primitive
// Similar to data but for integers.
func (event *_Event) number(key string, val int64) *_Event {
	invariant.Always(event != nil, "Callers of event.number don't propagate nil events")
	if key == "" {
		invariant.Sometimes(true, "event.number key is empty")
		key = EmptyIndicatorString
	}
	invariant.AlwaysNil(ValidateKey(stringToBytesUnsafe(key)), "Log context key is valid")
	invariant.Sometimes(true, "Add number context to event")

	event.Buffer = append(event.Buffer, stringToBytesUnsafe(key)...)
	event.Buffer = append(event.Buffer, KeyValDelimiter)
	event.Buffer = strconv.AppendInt(event.Buffer, val, 10)
	event.Buffer = append(event.Buffer, ComponentDelimiter)

	return event
}

// Primitive
// Exists separately from number because uint64 values may exceed int64 range.
func (event *_Event) Uint64(key string, val uint64) *_Event {
	if event == nil {
		invariant.Sometimes(true, "event.Uint64 _Event is nil")
		return nil
	}
	if key == "" {
		invariant.Sometimes(true, "event.Uint64 key is empty")
		key = EmptyIndicatorString
	}
	invariant.AlwaysNil(ValidateKey(stringToBytesUnsafe(key)), "Log context key is valid")
	invariant.Sometimes(true, "Add uint64 context to event")
	event.Buffer = append(event.Buffer, stringToBytesUnsafe(key)...)
	event.Buffer = append(event.Buffer, KeyValDelimiter)
	event.Buffer = strconv.AppendUint(event.Buffer, val, 10)
	event.Buffer = append(event.Buffer, ComponentDelimiter)
	return event
}

// Primitive
// With* functions create a deep copy of logger and appends context to the Buffer.
func (lgr *Logger) withData(key, val []byte) *Logger {
	if lgr == nil {
		invariant.Unreachable("logger.withData callers don't propagate nil loggers")
		return nil
	}
	if key == nil {
		invariant.Sometimes(true, "logger.withData callers don't propagate empty keys")
		key = EmptyIndicatorBytes
	}
	if val == nil {
		invariant.Unreachable("logger.withData callers don't propagate empty vals")
		val = EmptyIndicatorBytes
	}
	ValidateKey(key)
	invariant.Sometimes(true, "Add context to logger")
	invariant.Sometimes(len(lgr.Buffer) == 0, "Logger has no inheritable context")
	invariant.Sometimes(len(lgr.Buffer) > 0, "Logger already has inheritable context")

	dst := New(lgr.Writer, lgr.Level)

	// Assume that the inherited buffer was already processed by appendEscaped
	dst.Buffer = append(dst.Buffer, lgr.Buffer...)
	dst.Buffer = appendEscaped(dst.Buffer, key)
	dst.Buffer = append(dst.Buffer, KeyValDelimiter)
	dst.Buffer = appendEscaped(dst.Buffer, val)
	dst.Buffer = append(dst.Buffer, ComponentDelimiter)

	invariant.Always(dst.Buffer[0] != ComponentDelimiter, "Logger's context is appended AFTER ComponentDelimiter")
	return dst
}

// Primitive
// With* functions create a deep copy of logger and appends context to the Buffer.
func (logger *Logger) WithStr(key, val string) *Logger {
	if logger == nil {
		invariant.Sometimes(true, "logger.WithStr Logger is nil")
		return nil
	}
	if key == "" {
		invariant.Sometimes(true, "logger.WithStr key is empty")
		key = EmptyIndicatorString
	}
	if val == "" {
		invariant.Sometimes(true, "logger.WithStr val is empty")
		val = EmptyIndicatorString
	}
	invariant.AlwaysNil(ValidateKey(stringToBytesUnsafe(key)), "Log context key is valid")
	invariant.Sometimes(true, "Add context to logger")
	invariant.Sometimes(len(logger.Buffer) == 0, "Logger has no inheritable context")
	invariant.Sometimes(len(logger.Buffer) > 0, "Logger already has inheritable context")

	dst := New(logger.Writer, logger.Level)

	// Assume that the inherited buffer was already processed by appendEscaped
	dst.Buffer = append(dst.Buffer, logger.Buffer...)
	dst.Buffer = append(dst.Buffer, stringToBytesUnsafe(key)...)
	dst.Buffer = append(dst.Buffer, KeyValDelimiter)
	dst.Buffer = append(dst.Buffer, Quote)
	dst.Buffer = appendEscaped(dst.Buffer, stringToBytesUnsafe(val))
	dst.Buffer = append(dst.Buffer, Quote)
	dst.Buffer = append(dst.Buffer, ComponentDelimiter)

	invariant.Always(dst.Buffer[0] != ComponentDelimiter, "Logger's context is appended AFTER ComponentDelimiter")
	return dst
}

func New(writer io.Writer, level int) *Logger {
	if level >= LevelDisabled {
		invariant.Sometimes(true, "Logger is disabled completely")
		return nil
	}
	if writer == nil {
		invariant.Sometimes(true, "log Writer is nil")
		return nil
	}
	return &Logger{
		Writer: writer,
		Buffer: make([]byte, 0, DefaultLoggerBufferCapacity),
		Level:  level,
	}
}

func (logger *Logger) Debug() *_Event {
	if logger == nil {
		invariant.Sometimes(true, "logger.Debug Logger is nil")
		return nil
	} else if logger.Level > LevelDebug {
		invariant.Sometimes(true, "Debug level and below is disabled")
		return nil
	}
	invariant.Sometimes(true, "Create debug log")
	return logger.newEvent("DBG")
}

func (logger *Logger) Info() *_Event {
	if logger == nil {
		invariant.Sometimes(true, "logger.Info Logger is nil")
		return nil
	} else if logger.Level > LevelInfo {
		invariant.Sometimes(true, "Info level and below is disabled")
		return nil
	}
	invariant.Sometimes(true, "Create info log")
	return logger.newEvent("INF")
}

func (logger *Logger) Warn() *_Event {
	if logger == nil {
		invariant.Sometimes(true, "logger.Warn Logger is nil")
		return nil
	} else if logger.Level > LevelWarn {
		invariant.Sometimes(true, "Warn level and below is disabled")
		return nil
	}
	invariant.Sometimes(true, "Create warn log")
	return logger.newEvent("WRN")
}

// The errs parameter is mainly convenience. One benefit of it however, the
// error is ensured to be the first key value pair in the context if the parent
// logger doesn't have a context to inherit.
func (logger *Logger) Error(errs ...error) *_Event {
	if logger == nil {
		invariant.Sometimes(true, "logger.Error Logger is nil")
		return nil
	} else if logger.Level > LevelError {
		invariant.Sometimes(true, "logger.Error error level and below is disabled")
		return nil
	}
	event := logger.newEvent("ERR")
	switch len(errs) {
	case 0:
		invariant.Sometimes(true, "logger.Error has zero arguments")
		noop()
	case 1:
		invariant.Sometimes(true, "logger.Error has one argument")
		event = event.Err(errs[0])
	default:
		invariant.Sometimes(true, "logger.Error has multiple arguments")
		event = event.Errs(errs...)
	}
	return event
}

func (logger *Logger) WithTime(key string, t time.Time) *Logger {
	if logger == nil {
		invariant.Sometimes(true, "logger.WithTime Logger is nil")
		return nil
	}
	array := [TimestampCapacity]byte{}
	buf := array[:0]
	buf = appendTime(buf, t)
	return logger.withData(stringToBytesUnsafe(key), buf)
}

func (logger *Logger) WithErr(key string, val error) *Logger {
	if logger == nil {
		invariant.Sometimes(true, "logger.WithErr Logger is nil")
		return nil
	}
	if val != nil {
		invariant.Sometimes(true, "Add err context to logger")
		logger = logger.WithStr(key, val.Error())
	} else {
		invariant.Sometimes(true, "logger.WithErr got nil error")
	}
	return logger
}

func (ev *_Event) Float32(key string, val float32) *_Event {
	if ev == nil {
		invariant.Sometimes(true, "event.Float32 _Event is nil")
		return nil
	}
	invariant.Sometimes(true, "Add number context to logger")
	array := [33]byte{}
	buf := array[:0]
	buf = strconv.AppendFloat(buf, float64(val), 'f', -1, 32)
	return ev.data(stringToBytesUnsafe(key), buf)
}

func (logger *Logger) withNumber(key string, val int64) *Logger {
	if logger == nil {
		invariant.Unreachable("logger.withNumber callers don't propagate nil loggers")
		return nil
	}
	invariant.Sometimes(true, "Add number context to logger")
	array := [64]byte{}
	buf := array[:0]
	buf = strconv.AppendInt(buf, val, 10)
	return logger.withData(stringToBytesUnsafe(key), buf)
}

func (logger *Logger) WithInt8(key string, val int8) *Logger {
	if logger == nil {
		invariant.Sometimes(true, "logger.WithInt8 Logger is nil")
		return nil
	}
	invariant.Sometimes(true, "Add int8 context to logger")
	return logger.withNumber(key, int64(val))
}

func (logger *Logger) WithInt(key string, val int) *Logger {
	if logger == nil {
		invariant.Sometimes(true, "logger.WithInt Logger is nil")
		return nil
	}
	invariant.Sometimes(true, "Add int context to logger")
	return logger.withNumber(key, int64(val))
}

func (logger *Logger) WithUint64(key string, val uint64) *Logger {
	if logger == nil {
		invariant.Sometimes(true, "logger.withNumber Logger is nil")
		return nil
	}
	invariant.Sometimes(true, "Add number context to logger")
	array := [64]byte{}
	buf := array[:0]
	buf = strconv.AppendUint(buf, val, 10)
	return logger.withData(stringToBytesUnsafe(key), buf)
}

func (logger *Logger) WithBool(key string, cond bool) *Logger {
	if logger == nil {
		invariant.Sometimes(true, "logger.WithBool Logger is nil")
		return nil
	}
	val := []byte{'f', 'a', 'l', 's', 'e'}
	if cond {
		invariant.Sometimes(cond, "logger.WithBool cond is true")
		val = []byte{'t', 'r', 'u', 'e'}
	}
	invariant.Sometimes(!cond, "logger.WithBool cond is false")
	return logger.withData(stringToBytesUnsafe(key), val)
}

// Convenience functions to help guide your message. With these prefixes, you'd
// want to start with verbs in the present-progressive form.
// Usage:
//
//	logger.Info().Begin("")
func (event *_Event) Begin(msg string) {
	if event == nil {
		invariant.Sometimes(true, "event.Begin _Event is nil")
		return
	}
	event.Msg("begin " + msg)
}

// Don't use these like tracing logs. Instead of deferring unconditionally,
// manually write this as the very last statement of a function to indicate
// success. Otherwise, you probably have some WARN/ERROR log outputted
// beforehand, making it redundant.
func (event *_Event) Done(msg string) {
	if event == nil {
		invariant.Sometimes(true, "event.Done _Event is nil")
		return
	}
	event.Msg("done  " + msg)
}

func (event *_Event) Err(err error) *_Event {
	if event == nil {
		invariant.Sometimes(event == nil, "event.Err _Event is nil")
		return nil
	}
	if err != nil {
		invariant.Sometimes(true, "Add error context to event")
		event = event.Str("error", err.Error())
	} else {
		invariant.Sometimes(true, "event.Err got nil error")
	}
	return event
}

func (event *_Event) Errs(vals ...error) *_Event {
	invariant.Always(len(vals) > 0, "event.Errs takes at least one error")
	if event == nil {
		invariant.Sometimes(event == nil, "event.Errs _Event is nil")
		return nil
	}
	nilCount := 0
	for _, v := range vals {
		if v == nil {
			nilCount++
		} else {
			event = event.Err(v)
		}
	}
	invariant.Sometimes(nilCount == 0, "All arguments to event.<level>.Errs are non-nil")
	invariant.Sometimes(nilCount > 0, "Some arguments to event.<level>.Errs are non-nil")
	invariant.Sometimes(nilCount == len(vals), "All arguments to event.<level>.Errs are nil")
	return event
}

func (event *_Event) Int(key string, val int) *_Event {
	if event == nil {
		invariant.Sometimes(true, "event.Int _Event is nil")
		return nil
	}
	invariant.Sometimes(true, "Add int context to event")
	return event.number(key, int64(val))
}

func (event *_Event) Int8(key string, val int8) *_Event {
	if event == nil {
		invariant.Sometimes(true, "event.Int8 _Event is nil")
		return nil
	}
	invariant.Sometimes(true, "Add int8 context to event")
	return event.number(key, int64(val))
}

func (event *_Event) Int16(key string, val int16) *_Event {
	if event == nil {
		invariant.Sometimes(true, "event.Int16 _Event is nil")
		return nil
	}
	return event.number(key, int64(val))
}

func (event *_Event) Int32(key string, val int32) *_Event {
	if event == nil {
		invariant.Sometimes(true, "event.Int32 _Event is nil")
		return nil
	}
	invariant.Sometimes(true, "Add int32 context to event")
	return event.number(key, int64(val))
}

func (event *_Event) Int64(key string, val int64) *_Event {
	if event == nil {
		invariant.Sometimes(true, "event.Int64 _Event is nil")
		return nil
	}
	return event.number(key, int64(val))
}

func (event *_Event) Uint(key string, val uint) *_Event {
	if event == nil {
		invariant.Sometimes(true, "event.Uint _Event is nil")
		return nil
	}
	invariant.Sometimes(true, "Add uint context to event")
	return event.Uint64(key, uint64(val))
}

func (event *_Event) Uint8(key string, val uint8) *_Event {
	if event == nil {
		invariant.Sometimes(true, "event.Uint8 _Event is nil")
		return nil
	}
	invariant.Sometimes(true, "Add uint8 context to event")
	return event.number(key, int64(val))
}

func (event *_Event) Uint16(key string, val uint16) *_Event {
	if event == nil {
		invariant.Sometimes(true, "event.Uint16 _Event is nil")
		return nil
	}
	invariant.Sometimes(true, "Add uint16 context to event")
	return event.number(key, int64(val))
}

func (event *_Event) Uint32(key string, val uint32) *_Event {
	if event == nil {
		invariant.Sometimes(true, "event.Uint32 _Event is nil")
		return nil
	}
	invariant.Sometimes(true, "Add uint32 context to event")
	return event.number(key, int64(val))
}

func (logger *Logger) WithFloat32(key string, val float32) *Logger {
	if logger == nil {
		invariant.Sometimes(true, "logger.WithFloat32 Logger is nil")
		return nil
	}
	invariant.Sometimes(true, "Add number context to logger")
	array := [33]byte{}
	buf := array[:0]
	buf = strconv.AppendFloat(buf, float64(val), 'f', -1, 32)
	return logger.withData(stringToBytesUnsafe(key), buf)
}

func (event *_Event) Bool(key string, cond bool) *_Event {
	if event == nil {
		invariant.Sometimes(true, "event.Bool _Event is nil")
		return nil
	}
	val := "false"
	if cond {
		val = "true"
	}
	invariant.Sometimes(true, "Add bool context to event")
	invariant.Sometimes(val == "true", "event.Bool has value true")
	invariant.Sometimes(val == "false", "event.Bool has value false")
	return event.data(stringToBytesUnsafe(key), stringToBytesUnsafe(val))
}

func (event *_Event) Time(key string, t time.Time) *_Event {
	if event == nil {
		invariant.Sometimes(true, "event.Time _Event is nil")
		return nil
	}
	array := [TimestampCapacity]byte{}
	buf := array[:0]
	buf = appendTime(buf, t)
	return event.data(stringToBytesUnsafe(key), buf)
}

// Msg is a short summary of your log entry, similar to a git commit message.
// Msg asserts that msg does not contain a raw newline or raw null byte.
// If msg is longer than MessageCapacity, it gets truncated with no indicator.
func (event *_Event) Msg(msg string) {
	if event == nil {
		invariant.Sometimes(true, "event.Msg event is nil")
		return
	}
	defer event.destroy()

	invariant.Sometimes(len(msg) < MessageCapacity, "Message didn't fill the sub buffer")
	invariant.Sometimes(len(msg) == MessageCapacity, "Message fills the sub buffer exactly")
	invariant.Sometimes(len(msg) > MessageCapacity, "Message overfills the sub buffer")
	if msg == "" {
		invariant.Sometimes(true, "Log message is empty")
		msg = EmptyIndicatorString
	}

	// insert message
	{
		start := HeaderCapacity - MessageCapacity
		end := HeaderCapacity
		invariant.Always(len(event.Buffer) >= end, "Length is unsafely set past message buffer during event init")

		buf := event.Buffer[start:end]
		i := 0
		for ; i < min(len(buf), len(msg)); i++ {
			ch := msg[i]
			if ch == 0 || ch == '\n' {
				buf[i] = ' '
			} else {
				buf[i] = ch
			}
		}
		for ; i < len(buf); i++ {
			buf[i] = ' '
		}
	}

	// assert valid log
	{
		_, err := time.Parse(time.RFC3339, bytesToStringUnsafe(event.Buffer[:TimestampCapacity]))
		invariant.Always(err == nil, "Timestamp is valid RFC3339")
		invariant.Always(event.Buffer[TimestampCapacity] == ComponentDelimiter, "ComponentDelimiter found after timestamp")
		invariant.Always(func() bool {
			switch bytesToStringUnsafe(event.Buffer[TimestampCapacity+1 : TimestampCapacity+1+LevelCapacity]) {
			case "DBG", "INF", "WRN", "ERR":
				return true
			default:
				return false
			}
		}(), "Timestamp level word is either DBG, INF, WRN, or ERR")
		invariant.Always(event.Buffer[TimestampCapacity+1+LevelCapacity] == ComponentDelimiter, "ComponentDelimiter found after level word")

		{
			description := "Log message does not contain raw newlines or null bytes"
			invariant.XAlways(func() bool {
				for i := range event.Buffer[TimestampCapacity+1+LevelCapacity+1 : HeaderCapacity] {
					i += TimestampCapacity + 1 + LevelCapacity + 1
					ch := event.Buffer[i]
					if ch == '\n' {
						return false
					} else if ch == 0 {
						return false
					}
				}
				return true
			}, description)
		}

		{
			invariant.XAlways(func() bool {
				escaped := false
				for i := range event.Buffer[HeaderCapacity+1:] {
					i += HeaderCapacity + 1
					ch := event.Buffer[i]
					if ch == '\n' {
						return false
					} else if ch == '\\' {
						escaped = !escaped
						continue
					}
					escaped = false
				}
				return true
			}, "Log context contains raw newline")
		}
	}

	event.Buffer = append(event.Buffer, '\n')
	invariant.Always(event.Writer != nil, "A logger with a nil writer never initializes an event")
	n, err := event.Writer.Write(event.Buffer)
	if err != nil {
		os.Stderr.Write(stringToBytesUnsafe("WRITE_ERROR|could not write log event\n"))
	}

	invariant.Sometimes(n > DefaultEventBufferCapacity, "Log exceeded default buffer size")
}

func (logger *Logger) newEvent(level string) *_Event {
	invariant.Always(logger != nil, "Callers of logger.newEvent don't propagate nil loggers")
	invariant.Always(len(level) == LevelMaxWordLength, "Level string is equal to LevelMaxWordLength")
	invariant.Sometimes(level == "DBG", "New event is level DBG")
	invariant.Sometimes(level == "INF", "New event is level INF")
	invariant.Sometimes(level == "WRN", "New event is level WRN")
	invariant.Sometimes(level == "ERR", "New event is level ERR")

	event := EventPool.Get().(*_Event)
	invariant.Sometimes(len(event.Buffer) > 0, "sync.Pool reused _Event with leftover data")
	event.Buffer = event.Buffer[:0]
	event.Writer = logger.Writer

	t := TickCallback().UTC()
	// Append YYYY-MM-DD
	invariant.Always(len(event.Buffer) == 0, "Buffer was cleared before being written to")
	event.Buffer = appendTime(event.Buffer, t)
	event.Buffer = append(event.Buffer, ComponentDelimiter)
	invariant.Always(len(event.Buffer) == TimestampCapacity+1, "Wrote exactly N bytes for Timestamp+ComponentDelimiter")

	event.Buffer = append(event.Buffer, stringToBytesUnsafe(level)...)
	invariant.Always(len(event.Buffer) == TimestampCapacity+1+LevelCapacity, "Wrote exactly N bytes for Level")
	event.Buffer = append(event.Buffer, ComponentDelimiter)

	// Set the slice length past the Msg() portion, starting at the context.
	elementSize := int(unsafe.Sizeof(event.Buffer[0]))
	previous := (*reflect.SliceHeader)(unsafe.Pointer(&event.Buffer))
	current := reflect.SliceHeader{
		Data: previous.Data,
		Cap:  previous.Cap,
		Len:  (len(event.Buffer) + MessageCapacity) * elementSize,
	}
	event.Buffer = *(*[]byte)(unsafe.Pointer(&current))
	invariant.Always(len(event.Buffer) == HeaderCapacity, "Skipped past the message sub buffer during initialization")
	invariant.Always(len(event.Buffer)+1 < cap(event.Buffer), "Default buffer size is greater than HeaderCapacity+ComponentDelimiter")
	event.Buffer = append(event.Buffer, ComponentDelimiter)
	invariant.Always(event.Buffer[HeaderCapacity] == ComponentDelimiter, "Component separator after header was set during event initialization")
	event.Buffer = append(event.Buffer, logger.Buffer...)
	return event
}

func appendTime(dst []byte, t time.Time) []byte {
	append_zero_pad := func(buf []byte, v int) []byte {
		if v < 10 {
			buf = append(buf, '0')
		}
		return strconv.AppendInt(buf, int64(v), 10)
	}
	dst = strconv.AppendInt(dst, int64(t.Year()), 10)
	dst = append(dst, '-')
	dst = append_zero_pad(dst, int(t.Month()))
	dst = append(dst, '-')
	dst = append_zero_pad(dst, t.Day())
	dst = append(dst, 'T')
	dst = append_zero_pad(dst, t.Hour())
	dst = append(dst, ':')
	dst = append_zero_pad(dst, t.Minute())
	dst = append(dst, ':')
	dst = append_zero_pad(dst, t.Second())
	return append(dst, 'Z')
}

func (event *_Event) destroy() {
	if event == nil {
		invariant.Unreachable("event.Destroy caller does not propagate nil event")
		return
	}
	if cap(event.Buffer) > DefaultEventBufferCapacity {
		invariant.Sometimes(true, "_Event with oversized buffer isn't returned to the pool")
		noop()
	} else {
		EventPool.Put(event)
	}
}

// Logger is a long-lived object that primarily holds context data to be
// inherited by all of its child Events. All of Logger's methods that append to
// the context buffer create a new copy of Logger.
type Logger struct {
	Writer io.Writer
	// To be inherited by a _Event created by its methods.
	Buffer []byte
	Level  int
}

// _Event is a transient object that should not be touched after writing to
// Writer. Minimize scope as much as possible, usually in one statement. If you
// find yourself passing this as a function parameter, embed that context in the
// Logger instead. _Event methods modify the _Event itself through a pointer
// receiver.
type _Event struct {
	Writer io.Writer
	Buffer []byte
	// The log level is intentionally omitted from _Event. Logger.<Level>()
	// methods return nil if the event should not be logged, allowing method
	// chains like logger.Info().Str("key", "val").Msg("msg") to no-op
	// safely. This design eliminates the need to check the log level inside
	// _Event itself.
}

var EventPool = &sync.Pool{
	New: func() any {
		return &_Event{
			Buffer: make([]byte, 0, DefaultEventBufferCapacity),
		}
	},
}

func noop() {
}

func stringToBytesUnsafe(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

func bytesToStringUnsafe(b []byte) string {
	return unsafe.String(&b[0], len(b))
}
