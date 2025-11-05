/*
itlog is a performant, zero-allocation, logger (~500 SLoC) heavily inspired by ZeroLog (~16k SLoC). FWIW, this code was written AI-free :)

Advantages over Zerolog:

1. High-performance for human-readble output.
2. 30x smaller footprint

Design Decisions:

 1. Format

    |......Header.....|.Body..\n
    time|level|message|context\n

    The format is split into the Header and Body.
    The header is a fixed-width chunk, holding fields that are present in all logs. It is split into sub-components (1) time, (2) log level, (3) and
    message, all separated by the ComponentSeparator.
    The body is a variable-width chunk, holding dynamically appended fields. It consists of key=value pairs separated by ComponentSeparator and delimited by
    ContextKeyValueSeparator. Keys with multiple values are appended as individual pairs len(values) times in the context, sharing the same key.

    A key characteristic of this design is that it simplifies escaping. There are only two special characters, ComponentSeparator and ContextKeyValueSeparator.

 2. No colored output
    At first, I implemented colored output. In practice however, the colors are not all that useful when the context gets large and your terminal screen is
    filled with text that hard wrap, breaking visual alignment of the logs. It's better to save the logs in a file and explore them in your editor. Another
    benefit is that this simplifies the implementation further.

    TODO: Add a log parser.
    TODO: Add benchmarks.
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
)

// Not thread-safe
var Writer io.Writer = os.Stdout

const (
	DISABLE_ASSERTIONS    = false
	DefaultBufferCapacity = HeaderCapacity + 1 + ContextCapacity + len("\n")
	HeaderCapacity        = TimestampCapacity + 1 + LevelCapacity + 1 + MessageCapacity
	TimestampCapacity     = len(time.RFC3339) - len("07:00") // hardcoded to always be in UTC
	LevelCapacity         = LevelMaxWordLength
	// This should cover most cases. Note that these buffers can still grow when the need arises, in which case, they don't get returned to the
	// pool but are instead left for the garbage collector.
	MessageCapacity = 100
	ContextCapacity = 400

	LevelMaxWordLength = 5
	LevelDebug         = -10
	LevelInfo          = 0
	LevelWarn          = 10
	LevelError         = 20
	LevelDisabled      = 50

	// Note: This implementation assumes that these separators are characters and thus have a length of 1.
	ComponentSeparator       = '|'
	ContextKeyValueSeparator = '='
)

func appendAndEscape(dst *[]byte, src string) {
	for offset := 0; offset < len(src); offset += 1 {
		ch := src[offset]
		if ch == '=' || ch == ComponentSeparator {
			*dst = append(*dst, '\\')
		}
		*dst = append(*dst, ch)
	}
}

// Primitive
// With* functions append context to the logger.Buffer to be inherited by all of its child events.
func (logger *Logger) WithData(key, val string) *Logger {
	if logger == nil {
		return nil
	}
	dst := New(logger.Level)
	dst.Buffer = append(dst.Buffer, logger.Buffer...)
	appendAndEscape(&dst.Buffer, key)
	dst.Buffer = append(dst.Buffer, '=')
	appendAndEscape(&dst.Buffer, val)
	dst.Buffer = append(dst.Buffer, ComponentSeparator)
	assert(dst.Buffer[0] != ComponentSeparator, "Logger's inheritable context does not start with ComponentSeparator.")
	return dst
}

// Primitive
// Data appends a key-value pair to the Event context.
func (event *Event) Data(key, val string) *Event {
	if event == nil {
		return nil
	}
	appendAndEscape(&event.Buffer, key)
	event.Buffer = append(event.Buffer, '=')
	appendAndEscape(&event.Buffer, val)
	event.Buffer = append(event.Buffer, ComponentSeparator)
	return event
}

func New(level int) *Logger {
	if level >= LevelDisabled || Writer == nil {
		return nil
	}
	return &Logger{
		Buffer: make([]byte, 0, ContextCapacity/2),
		Level:  level,
	}
}

func (logger *Logger) Debug() *Event {
	if logger == nil || logger.Level > LevelDebug {
		return nil
	} else {
		return new_event(logger, "DEBUG")
	}
}

func (logger *Logger) Info() *Event {
	if logger == nil || logger.Level > LevelInfo {
		return nil
	} else {
		return new_event(logger, "INFO ")
	}
}

func (logger *Logger) Warn() *Event {
	if logger == nil || logger.Level > LevelWarn {
		return nil
	} else {
		return new_event(logger, "WARN ")
	}
}

// The errs parameter is mainly convenience. One benefit of it however, the error is ensured to be the first key value pair in the context IFF the parent logger
// already doesn't have a context to inherit.
func (logger *Logger) Error(errs ...error) *Event {
	if logger == nil || logger.Level > LevelError {
		return nil
	}
	event := new_event(logger, "ERROR")
	switch len(errs) {
	case 0:
		noop()
	case 1:
		event = event.Err(errs[0])
	default:
		event = event.Errs(errs...)
	}
	return event
}

// Convenience functions to help guide your message. With these prefixes, you'd want to start with verbs in the present-progressive form.
// Usage:
//
//	logger.Info().Begin("")
func (event *Event) Begin(msg string) {
	if event == nil {
		return
	}
	event.Msg("begin " + msg)
}

// Don't use these like tracing logs. Instead of deferring unconditionally, manually write this as the very last statement of a function to indicate success.
// Otherwise, you probably have some WARN/ERROR log outputted beforehand, making it redundant.
func (event *Event) Done(msg string) {
	if event == nil {
		return
	}
	event.Msg("done  " + msg)
}

func (logger *Logger) WithStr(key, val string) *Logger {
	if logger == nil {
		return nil
	}
	return logger.WithData(key, val)
}

func (logger *Logger) WithErr(key string, val error) *Logger {
	if logger == nil {
		return nil
	}
	if val != nil {
		logger = logger.WithData(key, val.Error())
	}
	return logger
}

func (logger *Logger) WithInt(key string, val int) *Logger {
	if logger == nil {
		return nil
	}
	return logger.WithData(key, strconv.Itoa(val))
}

func (logger *Logger) WithBool(key string, cond bool) *Logger {
	if logger == nil {
		return nil
	}
	val := "false"
	if cond {
		val = "true"
	}
	return logger.WithData(key, val)
}

func (event *Event) Str(key, val string) *Event {
	if event == nil {
		return nil
	}
	return event.Data(key, val)
}

func (event *Event) Err(err error) *Event {
	if event == nil {
		return nil
	}
	if err != nil {
		event = event.Data("error", err.Error())
	}
	return event
}

func (event *Event) Errs(vals ...error) *Event {
	if event == nil || len(vals) == 0 {
		return nil
	}
	for _, v := range vals {
		if v != nil {
			event = event.Data("error", v.Error())
		}
	}
	return event
}

func (event *Event) List(key string, vals ...string) *Event {
	if event == nil {
		return nil
	}
	for _, val := range vals {
		event = event.Data(key, val)
	}
	return event
}

func (event *Event) Int(key string, val int) *Event {
	if event == nil {
		return nil
	}
	return event.Number(key, int64(val))
}

func (event *Event) Int8(key string, val int8) *Event {
	if event == nil {
		return nil
	}
	return event.Number(key, int64(val))
}

func (event *Event) Int16(key string, val int16) *Event {
	if event == nil {
		return nil
	}
	return event.Number(key, int64(val))
}

func (event *Event) Int32(key string, val int32) *Event {
	if event == nil {
		return nil
	}
	return event.Number(key, int64(val))
}

func (event *Event) Int64(key string, val int64) *Event {
	if event == nil {
		return nil
	}
	return event.Number(key, int64(val))
}

func (event *Event) Uint(key string, val uint) *Event {
	if event == nil {
		return nil
	}
	return event.Uint64(key, uint64(val))
}

func (event *Event) Uint8(key string, val uint8) *Event {
	if event == nil {
		return nil
	}
	return event.Number(key, int64(val))
}

func (event *Event) Uint16(key string, val uint16) *Event {
	if event == nil {
		return nil
	}
	return event.Number(key, int64(val))
}

func (event *Event) Uint32(key string, val uint32) *Event {
	if event == nil {
		return nil
	}
	return event.Number(key, int64(val))
}

func (event *Event) Uint64(key string, val uint64) *Event {
	if event == nil {
		return nil
	}
	appendAndEscape(&event.Buffer, key)
	event.Buffer = append(event.Buffer, '=')
	event.Buffer = strconv.AppendUint(event.Buffer, val, 10)
	event.Buffer = append(event.Buffer, ComponentSeparator)
	return event
}

func (event *Event) Number(key string, val int64) *Event {
	if event == nil {
		return nil
	}
	appendAndEscape(&event.Buffer, key)
	event.Buffer = append(event.Buffer, '=')
	event.Buffer = strconv.AppendInt(event.Buffer, val, 10)
	event.Buffer = append(event.Buffer, ComponentSeparator)
	return event
}

func (event *Event) Bool(key string, cond bool) *Event {
	if event == nil {
		return nil
	}
	val := "false"
	if cond {
		val = "true"
	}
	return event.Data(key, val)
}

// Msg is a short summary of your log entry, similar to a git commit message.
// If msg is longer than 100 characters, it gets truncated with no indicator.
func (event *Event) Msg(msg string) {
	if event == nil {
		return
	}
	if msg == "" {
		msg = "<empty>"
	}
	defer destroy_event(event)
	start := HeaderCapacity - MessageCapacity
	end := HeaderCapacity
	assert(event.Buffer[start-1] == ComponentSeparator, "Message starts after component separator")
	assert(len(event.Buffer) >= end, "Length is unsafely set past the empty message sub buffer during event initialization.")
	assert(event.Buffer[end] == ComponentSeparator, "Component separator after message was set during event initialization.")
	buf := event.Buffer[start:end]
	n := copy(buf, msg)
	for i := range buf[n:] {
		buf[n+i] = ' '
	}

	event.Buffer = append(event.Buffer, '\n')
	if _, err := Writer.Write(event.Buffer); err != nil {
		os.Stderr.Write(stringToBytes("WRITE_ERROR|could not write log event"))
		return
	}

	// assert after writing to make it more convenient to see what event.Buffer contained
	iAfterTime := TimestampCapacity + 1
	iAfterLevel := iAfterTime + LevelCapacity + 1
	iAfterMessage := iAfterLevel + MessageCapacity + 1

	assert(event.Buffer[iAfterTime-1] == ComponentSeparator, "Time component ends with ComponentSeparator")
	assert(event.Buffer[iAfterLevel-1] == ComponentSeparator, "Level component ends with ComponentSeparator")
	assert(event.Buffer[iAfterMessage-1] == ComponentSeparator, "Message component ends with ComponentSeparator")
	assert(func() bool {
		for i := range event.Buffer[:HeaderCapacity] {
			if event.Buffer[i] == 0 {
				return false
			}
		}
		return true
		// TODO: Do tests for messages, or contexts containing null bytes.
	}(), "Header does not contain null bytes.")
}

func new_event(logger *Logger, level string) *Event {
	assert(len(level) == LevelMaxWordLength, "Level string is equal to LevelMaxWordLength")
	if logger == nil {
		return nil
	}
	event := EventPool.Get().(*Event)
	sometimes(len(event.Buffer) > 0, "sync.Pool can return reused Events with leftover data")
	event.Buffer = event.Buffer[:0]

	append_zero_padded_int := func(buf []byte, v int) []byte {
		if v < 10 {
			buf = append(buf, '0')
		}
		return strconv.AppendInt(buf, int64(v), 10)
	}
	t := time.Now().UTC()
	// Append YYYY-MM-DD
	assert(len(event.Buffer) == 0, "Buffer was before cleared before being written to.")
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
	assert(len(event.Buffer) == TimestampCapacity+1, "Wrote exactly N bytes for Timestamp+ComponentSeparator.")

	event.Buffer = append(event.Buffer, stringToBytes(level)...)
	assert(len(event.Buffer) == TimestampCapacity+1+LevelCapacity, "Wrote exactly N bytes for Level.")
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
	assert(len(event.Buffer) == HeaderCapacity, "Skipped past the message sub buffer during initialization.")
	assert(len(event.Buffer)+1 < cap(event.Buffer), "Default buffer size is greater than HeaderCapacity+ComponentSeparator.")
	event.Buffer = append(event.Buffer, ComponentSeparator)
	assert(event.Buffer[HeaderCapacity] == ComponentSeparator, "Component separator after header was set during event initialization.")
	event.Buffer = append(event.Buffer, logger.Buffer...)
	return event
}

func destroy_event(event *Event) {
	if cap(event.Buffer) > DefaultBufferCapacity {
		return
	}
	EventPool.Put(event)
}

// Logger is a long-lived object that primarily holds context data to be inherited by all of its child Events.
// All of Logger's methods that append to the context buffer create a new copy of Logger.
type Logger struct {
	// To be inherited by a Event created by its methods.
	Buffer []byte
	Level  int
}

// Event is a transient object that should not be touched after writing to Writer. Minimize scope as much as possible, usually in one statement.
// If you find yourself passing this as a function parameter, embed that context in the Logger instead.
// Event methods modify the Event itself through a pointer receiver.
type Event struct {
	Buffer []byte
	// The log level is intentionally omitted from Event.
	// Logger.<Level>() methods return nil if the event should not be logged, allowing method chains like logger.Info().Str("key", "val").Msg("msg") to
	// no-op safely. This design eliminates the need to check the log level inside Event itself.
}

var EventPool = &sync.Pool{
	New: func() any {
		return &Event{
			Buffer: make([]byte, 0, DefaultBufferCapacity),
		}
	},
}

func noop() {
}

func sometimes(cond bool, msg string) {
	assert(cond || !cond, msg)
}

func assert(cond bool, msg string) {
	if !cond && !DISABLE_ASSERTIONS {
		os.Stderr.Write(stringToBytes("assertion failed: " + msg))
		os.Exit(1)
	}
}

func stringToBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
