/*
itlog is a performant, zero-allocation, logger (~600 SLoC) inspired by ZeroLog (~16k SLoC).
FWIW, this code was written AI-free :)

Design Decisions:

 1. Format

    .......Header.....|.Body..\n
    time|level|message|context\n

    The format is split into the Header and Body. The header is a fixed-width
    chunk, holding fields that are present in all logs. It is split into
    sub-components (1) time, (2) log level, (3) and message, all separated by
    the ComponentSeparator. The body is a variable-width chunk, holding
    dynamically appended fields. It consists of key=value pairs separated by
    ComponentSeparator and delimited by KeyValSeparator. Keys with
    multiple values are appended as individual pairs len(values) times in the
    context, sharing the same key.

    A key characteristic of this design is that it simplifies escaping. There
    are only four special characters to escape.

    - ComponentSeparator
    - KeyValSeparator
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
	"io"
	"os"
	"reflect"
	"strconv"
	"sync"
	"time"
	"unsafe"

	"golang_snacks/invariant"
)

// Not thread-safe
var Writer io.Writer = os.Stdout

const (
	DefaultLoggerBufferCapacity = DefaultEventBufferCapacity / 2
	DefaultEventBufferCapacity  = HeaderCapacity + 1 + ContextCapacity + len("\n")
	HeaderCapacity              = TimestampCapacity + 1 + LevelCapacity + 1 + MessageCapacity

	TimestampCapacity = len(time.RFC3339) - len("07:00") // hardcoded to always be in UTC
	LevelCapacity     = LevelMaxWordLength
	// You can get a 10ns/op improvement if you reduce this down to 50
	// characters but it's not worth it for such a short message window.
	MessageCapacity = 80
	// This should cover most cases. Note that the buffers can still grow
	// when the need arises, in which case, they don't get returned to the
	// pool but are instead left for the garbage collector.
	ContextCapacity = 300

	LevelMaxWordLength = 3
	LevelDebug         = -10
	LevelInfo          = 0
	LevelWarn          = 10
	LevelError         = 20
	LevelDisabled      = 50

	// Note: This implementation assumes that these separators are
	// characters and thus have a length of 1.
	ComponentSeparator             = '|'
	KeyValSeparator                = '='
	ComponentSeparatorEscapedAlias = ComponentSeparator
	KeyValSeparatorEscapedAlias    = KeyValSeparator
)

// === Encoding ===
// encodeThenAppend appends `src` to `dst`, escaping reserved characters so that
// the encoding is fully reversible.
//
// It first double-escapes existing backslashes in the data (turning `\` into `\\`)
// to distinguish between characters that were already escaped and those that will
// be escaped during this process.
//
// Then it replaces non-human-readable reserved characters:
// - newline   (0x0A)    -> `\n`
// - nullbyte  (0x00)    -> `\0`
// - key_value_separator -> `\<alias>`
// - component_separator -> `\<alias>`
//
// Rationale:
// Since raw newlines (0x0A) are turned into the string `\n`, we must
// distinguish them from literal `\n` sequences already present in the data
// to make the encoding reversible. Moreover, this ensures that the only
// raw newlines in the logs are log record delimiters; this is imperative
// for preserving visual structure. It prevents user data containing newlines
// from visually splitting a single record into multiple lines when displayed
// in GUIs.
//
// Null bytes - every null byte in the data is escaped, ensuring that any
// unescaped null byte appearing in the logs can be safely interpreted as data
// corruption.
//
// Notes:
// Other non-readable characters remain unchanged.
// UTF is not handled.
//
// The resulting encoding is lossless — it can be decoded back to the original data.
func encodeThenAppend(dst *[]byte, src string) {
	invariant.Always(dst != nil, "encodeThenAppend must receive a non-nil slice pointer")
	invariant.Always(src != "", "encodeThenAppend on an empty string is probably a programming error")
	before := len(*dst)
	defer func() {
		after := len(*dst)
		invariant.Sometimes(after-before == len(src), "String to encode was appended as is")
		invariant.Sometimes(after-before > len(src), "String to encode contained escaped characters")
	}()
	escaped := false
	for i := 0; i < len(src); i++ {
		ch := src[i]
		if escaped {
			invariant.Sometimes(true, "String to encode contains escaped characters")
			*dst = append(*dst, ch)
			escaped = false
			continue
		}

		if ch == '\\' {
			invariant.Sometimes(true, "String to encode contains raw backslash")
			escaped = true
			*dst = append(*dst, '\\', '\\')
			continue
		}

		switch ch {
		case KeyValSeparator:
			invariant.Sometimes(true, "String to encode contains raw KeyValSeparator")
			*dst = append(*dst, '\\', KeyValSeparatorEscapedAlias)
		case ComponentSeparator:
			invariant.Sometimes(true, "String to encode contains raw ComponentSeparator")
			*dst = append(*dst, '\\', ComponentSeparatorEscapedAlias)
		case '\n':
			invariant.Sometimes(true, "String to encode contains raw newline")
			*dst = append(*dst, '\\', 'n')
		case 0:
			invariant.Sometimes(true, "String to encode contains raw null byte")
			*dst = append(*dst, '\\', 0)
		default:
			*dst = append(*dst, ch)
		}
	}
}

// Primitive
// Data appends a key-value pair to the Event context.
func (event *Event) Data(key, val string) *Event {
	if event == nil {
		invariant.Sometimes(true, "Event is nil")
		return nil
	}
	invariant.Sometimes(true, "Add context to event")
	encodeThenAppend(&event.Buffer, key)
	event.Buffer = append(event.Buffer, KeyValSeparator)
	encodeThenAppend(&event.Buffer, val)
	event.Buffer = append(event.Buffer, ComponentSeparator)
	return event
}

// Primitive
// Conceptually similar to Data but for integers.
func (event *Event) Number(key string, val int64) *Event {
	if event == nil {
		invariant.Unreachable("This is so deep in the stack it will never happen")
		return nil
	}
	invariant.Sometimes(true, "Add number context to event")
	encodeThenAppend(&event.Buffer, key)
	event.Buffer = append(event.Buffer, KeyValSeparator)
	event.Buffer = strconv.AppendInt(event.Buffer, val, 10)
	event.Buffer = append(event.Buffer, ComponentSeparator)
	return event
}

// Primitive
// Exists separately from Number because uint64 values may exceed int64 range.
func (event *Event) Uint64(key string, val uint64) *Event {
	if event == nil {
		invariant.Sometimes(true, "Event is nil")
		return nil
	}
	invariant.Sometimes(true, "Add uint64 context to event")
	encodeThenAppend(&event.Buffer, key)
	event.Buffer = append(event.Buffer, KeyValSeparator)
	event.Buffer = strconv.AppendUint(event.Buffer, val, 10)
	event.Buffer = append(event.Buffer, ComponentSeparator)
	return event
}

// Primitive
// With* functions create a deep copy of logger and appends context to the Buffer.
func (logger *Logger) WithData(key, val string) *Logger {
	if logger == nil {
		invariant.Sometimes(true, "Logger is nil")
		return nil
	}
	invariant.Sometimes(true, "Add context to logger")
	invariant.Sometimes(len(logger.Buffer) == 0, "Logger has no inheritable context")
	invariant.Sometimes(len(logger.Buffer) > 0, "Logger already has inheritable context")
	dst := New(logger.Level)
	// Assume that the inherited buffer was already processed by encodeThenAppend
	dst.Buffer = append(dst.Buffer, logger.Buffer...)
	encodeThenAppend(&dst.Buffer, key)
	dst.Buffer = append(dst.Buffer, KeyValSeparator)
	encodeThenAppend(&dst.Buffer, val)
	dst.Buffer = append(dst.Buffer, ComponentSeparator)
	invariant.Always(dst.Buffer[0] != ComponentSeparator, "Logger's context is appended AFTER ComponentSeparator")
	return dst
}

func New(level int) *Logger {
	if level >= LevelDisabled {
		invariant.Sometimes(true, "Logger is disabled completely")
		return nil
	}
	if Writer == nil {
		invariant.Sometimes(true, "log Writer is nil")
		return nil
	}
	invariant.Sometimes(true, "Initialize logger via New()")
	return &Logger{
		Buffer: make([]byte, 0, DefaultLoggerBufferCapacity),
		Level:  level,
	}
}

func (logger *Logger) Debug() *Event {
	if logger == nil {
		invariant.Sometimes(true, "Logger is nil")
		return nil
	} else if logger.Level > LevelDebug {
		invariant.Sometimes(true, "Debug level and below is disabled")
		return nil
	}
	invariant.Sometimes(true, "Create debug log")
	return logger.NewEvent("DBG")
}

func (logger *Logger) Info() *Event {
	if logger == nil {
		invariant.Sometimes(true, "Logger is nil")
		return nil
	} else if logger.Level > LevelInfo {
		invariant.Sometimes(true, "Info level and below is disabled")
		return nil
	}
	invariant.Sometimes(true, "Create info log")
	return logger.NewEvent("INF")
}

func (logger *Logger) Warn() *Event {
	if logger == nil {
		invariant.Sometimes(true, "Logger is nil")
		return nil
	} else if logger.Level > LevelWarn {
		invariant.Sometimes(true, "Warn level and below is disabled")
		return nil
	}
	invariant.Sometimes(true, "Create warn log")
	return logger.NewEvent("WRN")
}

// The errs parameter is mainly convenience. One benefit of it however, the
// error is ensured to be the first key value pair in the context if the parent
// logger doesn't have a context to inherit.
func (logger *Logger) Error(errs ...error) *Event {
	if logger == nil {
		invariant.Sometimes(true, "Logger is nil")
		return nil
	} else if logger.Level > LevelError {
		invariant.Sometimes(true, "Error level and below is disabled")
		return nil
	}
	event := logger.NewEvent("ERR")
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

func (logger *Logger) WithStr(key, val string) *Logger {
	if logger == nil {
		invariant.Sometimes(true, "Logger is nil")
		return nil
	}
	invariant.Sometimes(true, "Add str context to logger")
	return logger.WithData(key, val)
}

func (logger *Logger) WithErr(key string, val error) *Logger {
	if logger == nil {
		invariant.Sometimes(true, "Logger is nil")
		return nil
	}
	if val != nil {
		invariant.Sometimes(true, "Add err context to logger")
		logger = logger.WithData(key, val.Error())
	} else {
		invariant.Sometimes(true, "WithErr got nil error")
	}
	return logger
}

func (logger *Logger) WithInt(key string, val int) *Logger {
	if logger == nil {
		invariant.Sometimes(true, "Logger is nil")
		return nil
	}
	invariant.Sometimes(true, "Add int context to logger")
	return logger.WithData(key, strconv.Itoa(val))
}

func (logger *Logger) WithBool(key string, cond bool) *Logger {
	if logger == nil {
		invariant.Sometimes(true, "Logger is nil")
		return nil
	}
	val := "false"
	if cond {
		val = "true"
	}
	invariant.Sometimes(true, "Add bool context to logger")
	invariant.Sometimes(val == "true", "WithBool cond is true")
	invariant.Sometimes(val == "false", "WithBool cond is false")
	return logger.WithData(key, val)
}

// Convenience functions to help guide your message. With these prefixes, you'd
// want to start with verbs in the present-progressive form.
// Usage:
//
//	logger.Info().Begin("")
func (event *Event) Begin(msg string) {
	if event == nil {
		invariant.Sometimes(true, "Event is nil")
		return
	}
	invariant.Sometimes(true, "Use convenience function: Begin()")
	event.Msg("begin " + msg)
}

// Don't use these like tracing logs. Instead of deferring unconditionally,
// manually write this as the very last statement of a function to indicate
// success. Otherwise, you probably have some WARN/ERROR log outputted
// beforehand, making it redundant.
func (event *Event) Done(msg string) {
	if event == nil {
		invariant.Sometimes(true, "Event is nil")
		return
	}
	invariant.Sometimes(true, "Use convenience function: Done()")
	event.Msg("done  " + msg)
}

func (event *Event) Str(key, val string) *Event {
	if event == nil {
		invariant.Sometimes(event == nil, "Event is nil")
		return nil
	}
	invariant.Sometimes(true, "Add string context to event")
	return event.Data(key, val)
}

func (event *Event) Err(err error) *Event {
	if event == nil {
		invariant.Sometimes(event == nil, "Event is nil")
		return nil
	}
	if err != nil {
		invariant.Sometimes(true, "Add error context to event")
		event = event.Data("error", err.Error())
	} else {
		invariant.Sometimes(true, "event.<level>().Err() got nil error")
	}
	return event
}

func (event *Event) Errs(vals ...error) *Event {
	invariant.Always(len(vals) > 0, "Errs takes at least one error")
	if event == nil {
		invariant.Sometimes(event == nil, "Event is nil")
		return nil
	}
	nilCount := 0
	for _, v := range vals {
		if v == nil {
			nilCount++
		} else {
			event = event.Data("error", v.Error())
		}
	}
	invariant.Sometimes(nilCount == 0, "All arguments to event.<level>.Errs are non-nil")
	invariant.Sometimes(nilCount > 0, "Some arguments to event.<level>.Errs are non-nil")
	invariant.Sometimes(nilCount == len(vals), "All arguments to event.<level>.Errs are nil")
	return event
}

func (event *Event) List(key string, vals ...string) *Event {
	if event == nil {
		invariant.Sometimes(true, "Event is nil")
		return nil
	}
	if len(vals) == 0 {
		invariant.Sometimes(true, "Forgot to add values to event.<level>.List")
		event = event.Data(key, "<forgot to add values to event.<level>.List()>")
	} else {
		for _, val := range vals {
			event = event.Data(key, val)
		}
	}
	return event
}

func (event *Event) Int(key string, val int) *Event {
	if event == nil {
		invariant.Sometimes(true, "Event is nil")
		return nil
	}
	invariant.Sometimes(true, "Add int context to event")
	return event.Number(key, int64(val))
}

func (event *Event) Int8(key string, val int8) *Event {
	if event == nil {
		invariant.Sometimes(true, "Event is nil")
		return nil
	}
	invariant.Sometimes(true, "Add int8 context to event")
	return event.Number(key, int64(val))
}

func (event *Event) Int16(key string, val int16) *Event {
	if event == nil {
		invariant.Sometimes(true, "Event is nil")
		return nil
	}
	return event.Number(key, int64(val))
}

func (event *Event) Int32(key string, val int32) *Event {
	if event == nil {
		invariant.Sometimes(true, "Event is nil")
		return nil
	}
	invariant.Sometimes(true, "Add int32 context to event")
	return event.Number(key, int64(val))
}

func (event *Event) Int64(key string, val int64) *Event {
	if event == nil {
		invariant.Sometimes(true, "Event is nil")
		return nil
	}
	return event.Number(key, int64(val))
}

func (event *Event) Uint(key string, val uint) *Event {
	if event == nil {
		invariant.Sometimes(true, "Event is nil")
		return nil
	}
	invariant.Sometimes(true, "Add uint context to event")
	return event.Uint64(key, uint64(val))
}

func (event *Event) Uint8(key string, val uint8) *Event {
	if event == nil {
		invariant.Sometimes(true, "Event is nil")
		return nil
	}
	invariant.Sometimes(true, "Add uint8 context to event")
	return event.Number(key, int64(val))
}

func (event *Event) Uint16(key string, val uint16) *Event {
	if event == nil {
		invariant.Sometimes(true, "Event is nil")
		return nil
	}
	invariant.Sometimes(true, "Add uint16 context to event")
	return event.Number(key, int64(val))
}

func (event *Event) Uint32(key string, val uint32) *Event {
	if event == nil {
		invariant.Sometimes(true, "Event is nil")
		return nil
	}
	invariant.Sometimes(true, "Add uint32 context to event")
	return event.Number(key, int64(val))
}

func (event *Event) Bool(key string, cond bool) *Event {
	if event == nil {
		invariant.Sometimes(true, "Event is nil")
		return nil
	}
	val := "false"
	if cond {
		val = "true"
	}
	invariant.Sometimes(true, "Add bool context to event")
	invariant.Sometimes(val == "true", "event.Bool has value true")
	invariant.Sometimes(val == "false", "event.Bool has value false")
	return event.Data(key, val)
}

// Msg is a short summary of your log entry, similar to a git commit message.
// If msg is longer than 100 characters, it gets truncated with no indicator.
func (event *Event) Msg(msg string) {
	defer event.destroy()
	if event == nil {
		invariant.Sometimes(true, "Event is nil")
		return
	}
	if msg == "" {
		invariant.Sometimes(true, "Log message is empty")
		msg = "<empty>"
	}

	{
		start := HeaderCapacity - MessageCapacity
		end := HeaderCapacity
		invariant.Always(event.Buffer[start-1] == ComponentSeparator, "Message starts after component separator")
		invariant.Always(len(event.Buffer) >= end, "Length is unsafely set past message buffer during event init")
		invariant.Always(event.Buffer[end] == ComponentSeparator, "Component separator after message was set during event init")

		buf := event.Buffer[start:end]
		n := copy(buf, msg)
		invariant.Sometimes(n < len(buf), "Message didn't fill the sub buffer")
		invariant.Sometimes(n == len(buf), "Message filled the sub buffer completely")
		for i := range buf[n:] {
			buf[n+i] = ' '
		}
	}

	event.Buffer = append(event.Buffer, '\n')
	n, err := Writer.Write(event.Buffer)
	// TODO: do a random int!
	if err != nil {
		os.Stderr.Write(stringToBytesUnsafe("WRITE_ERROR|could not write log event"))
		return
	}
	invariant.Sometimes(n > DefaultEventBufferCapacity, "Log exceeded default buffer size")

	// assert after writing to make it more convenient to see what event.Buffer contained
	idxAfterTime := TimestampCapacity + 1
	idxAfterLevel := idxAfterTime + LevelCapacity + 1
	idxAfterMessage := idxAfterLevel + MessageCapacity + 1

	invariant.Always(event.Buffer[idxAfterTime-1] == ComponentSeparator, "Time component ends with ComponentSeparator")
	invariant.Always(event.Buffer[idxAfterLevel-1] == ComponentSeparator, "Level component ends with ComponentSeparator")
	invariant.Always(event.Buffer[idxAfterMessage-1] == ComponentSeparator, "Message component ends with ComponentSeparator")
	invariant.XSometimes(
		func() bool {
			for i := range event.Buffer[:n] {
				if event.Buffer[i] == 0 {
					return true
				}
			}
			return false
		},
		"Log is corrupted",
	)
}

func (logger *Logger) NewEvent(level string) *Event {
	invariant.Always(len(level) == LevelMaxWordLength, "Level string is equal to LevelMaxWordLength")
	invariant.Sometimes(level == "DBG", "New event is level DBG")
	invariant.Sometimes(level == "INF", "New event is level INF")
	invariant.Sometimes(level == "WRN", "New event is level WRN")
	invariant.Sometimes(level == "ERR", "New event is level ERR")
	if logger == nil {
		invariant.Sometimes(true, "Logger is nil")
		return nil
	}
	event := EventPool.Get().(*Event)
	invariant.Sometimes(len(event.Buffer) > 0, "sync.Pool can return reused Events with leftover data")
	event.Buffer = event.Buffer[:0]

	append_zero_padded_int := func(buf []byte, v int) []byte {
		if v < 10 {
			buf = append(buf, '0')
		}
		return strconv.AppendInt(buf, int64(v), 10)
	}
	t := time.Now().UTC()
	// Append YYYY-MM-DD
	invariant.Always(len(event.Buffer) == 0, "Buffer was before cleared before being written to")
	event.Buffer = strconv.AppendInt(event.Buffer, int64(t.Year()), 10)
	event.Buffer = append(event.Buffer, '-')
	event.Buffer = append_zero_padded_int(event.Buffer, int(t.Month()))
	event.Buffer = append(event.Buffer, '-')
	event.Buffer = append_zero_padded_int(event.Buffer, t.Day())
	// Append THH:mm:ss
	event.Buffer = append(event.Buffer, 'T')
	event.Buffer = append_zero_padded_int(event.Buffer, t.Hour())
	event.Buffer = append(event.Buffer, ':')
	event.Buffer = append_zero_padded_int(event.Buffer, t.Minute())
	event.Buffer = append(event.Buffer, ':')
	event.Buffer = append_zero_padded_int(event.Buffer, t.Second())
	event.Buffer = append(event.Buffer, 'Z', ComponentSeparator)
	invariant.Always(len(event.Buffer) == TimestampCapacity+1, "Wrote exactly N bytes for Timestamp+ComponentSeparator")

	event.Buffer = append(event.Buffer, stringToBytesUnsafe(level)...)
	invariant.Always(len(event.Buffer) == TimestampCapacity+1+LevelCapacity, "Wrote exactly N bytes for Level")
	event.Buffer = append(event.Buffer, ComponentSeparator)

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
	invariant.Always(len(event.Buffer)+1 < cap(event.Buffer), "Default buffer size is greater than HeaderCapacity+ComponentSeparator")
	event.Buffer = append(event.Buffer, ComponentSeparator)
	invariant.Always(event.Buffer[HeaderCapacity] == ComponentSeparator, "Component separator after header was set during event initialization")
	event.Buffer = append(event.Buffer, logger.Buffer...)
	return event
}

func (event *Event) destroy() {
	if event == nil {
		invariant.Sometimes(true, "Event is nil")
		return
	}
	if cap(event.Buffer) > DefaultEventBufferCapacity {
		invariant.Sometimes(true, "Event with oversized buffer isn't returned to the pool")
		return
	}
	EventPool.Put(event)
}

// Logger is a long-lived object that primarily holds context data to be
// inherited by all of its child Events. All of Logger's methods that append to
// the context buffer create a new copy of Logger.
type Logger struct {
	// To be inherited by a Event created by its methods.
	Buffer []byte
	Level  int
}

// Event is a transient object that should not be touched after writing to
// Writer. Minimize scope as much as possible, usually in one statement. If you
// find yourself passing this as a function parameter, embed that context in the
// Logger instead. Event methods modify the Event itself through a pointer
// receiver.
type Event struct {
	Buffer []byte
	// The log level is intentionally omitted from Event. Logger.<Level>()
	// methods return nil if the event should not be logged, allowing method
	// chains like logger.Info().Str("key", "val").Msg("msg") to no-op
	// safely. This design eliminates the need to check the log level inside
	// Event itself.
}

// TODO: Do a bounded pool
var EventPool = &sync.Pool{
	New: func() any {
		return &Event{
			Buffer: make([]byte, 0, DefaultEventBufferCapacity),
		}
	},
}

func noop() {
}

func stringToBytesUnsafe(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
