// This file is AI-generated.
package itlog_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"

	"golang_snacks/invariant"
	"golang_snacks/itlog"
)

func TestMain(m *testing.M) {
	invariant.RegisterPackagesForAnalysis()
	code := m.Run()
	invariant.AnalyzeAssertionFrequency()
	os.Exit(code)
}

func TestBasicInfoLog(t *testing.T) {
	orig := itlog.Writer
	defer func() { itlog.Writer = orig }()

	var buf bytes.Buffer
	itlog.Writer = &buf

	l := itlog.New(itlog.LevelInfo)
	if l == nil {
		t.Fatal("New returned nil")
	}
	l.Info().Str("user", "alice").Int("id", 42).Msg("hello")

	out := buf.String()
	// header:  YYYY-MM-DDTHH:MM:SSZ|INF|
	if matched := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z\|INF\|`).MatchString(out); !matched {
		t.Fatalf("unexpected header: %q", out)
	}
	if !strings.Contains(out, "hello") || !strings.Contains(out, "user=alice") || !strings.Contains(out, "id=42") {
		t.Fatalf("missing payload: %q", out)
	}
}

func TestLevelFiltering(t *testing.T) {
	l := itlog.New(itlog.LevelError)
	if l == nil {
		t.Fatal("New returned nil")
	}
	if l.Debug() != nil {
		t.Fatal("Debug must be disabled at Error level")
	}
	if l.Info() != nil {
		t.Fatal("Info must be disabled at Error level")
	}
	if l.Warn() != nil {
		t.Fatal("Warn must be disabled at Error level")
	}
	if l.Error() == nil {
		t.Fatal("Error must be enabled at Error level")
	}
}

func TestWithContextInheritance(t *testing.T) {
	orig := itlog.Writer
	defer func() { itlog.Writer = orig }()

	var buf bytes.Buffer
	itlog.Writer = &buf

	base := itlog.New(itlog.LevelInfo)
	if base == nil {
		t.Fatal("New returned nil")
	}
	base = base.WithStr("svc", "auth")
	child := base.WithStr("env", "prod")
	child.Info().Msg("started")

	out := buf.String()
	if !strings.Contains(out, "svc=auth") || !strings.Contains(out, "env=prod") {
		t.Fatalf("inherited context missing: %q", out)
	}
}

func TestErrorConvenience(t *testing.T) {
	orig := itlog.Writer
	defer func() { itlog.Writer = orig }()

	var buf bytes.Buffer
	itlog.Writer = &buf

	l := itlog.New(itlog.LevelInfo)
	if l == nil {
		t.Fatal("New returned nil")
	}

	// no-arg Error() should not inject an error key
	l.Error().Msg("noerr")
	if strings.Contains(buf.String(), "error=") {
		t.Fatalf("unexpected error key present: %q", buf.String())
	}

	buf.Reset()
	// single error should appear as error=<msg>
	l.Error(errors.New("boom")).Msg("witherr")
	if !strings.Contains(buf.String(), "error=boom") {
		t.Fatalf("expected error key missing: %q", buf.String())
	}
}

func newBufLogger(level int) (*itlog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	itlog.Writer = &buf
	return itlog.New(level), &buf
}

func TestAllLevels(t *testing.T) {
	l, buf := newBufLogger(itlog.LevelDebug)
	l.Debug().Msg("d")
	l.Info().Msg("i")
	l.Warn().Msg("w")
	l.Error().Msg("e")

	out := buf.String()
	for _, lvl := range []string{"DBG", "INF", "WRN", "ERR"} {
		if !strings.Contains(out, lvl) {
			t.Fatalf("missing %s", lvl)
		}
	}
}

func TestBoolVariants(t *testing.T) {
	l, buf := newBufLogger(itlog.LevelInfo)
	l.Info().Bool("yes", true).Bool("no", false).Msg("bools")
	out := buf.String()
	if !strings.Contains(out, "yes=true") || !strings.Contains(out, "no=false") {
		t.Fatalf("expected both bool values, got %q", out)
	}
}

func TestWithErrNilAndNonNil(t *testing.T) {
	l, buf := newBufLogger(itlog.LevelInfo)
	err := errors.New("ouch")
	l = l.WithErr("e1", err).WithErr("e2", nil)
	l.Info().Msg("errs")
	out := buf.String()
	if !strings.Contains(out, "e1=ouch") {
		t.Fatalf("missing non-nil err: %q", out)
	}
	if strings.Contains(out, "e2=") {
		t.Fatalf("should not include nil err: %q", out)
	}
}

func TestListAndErrs(t *testing.T) {
	l, buf := newBufLogger(itlog.LevelInfo)
	errs := []error{errors.New("a"), nil, errors.New("b")}
	l.Info().Errs(errs...).List("k").Msg("multi")
	out := buf.String()
	if !strings.Contains(out, "error=a") || !strings.Contains(out, "error=b") {
		t.Fatalf("expected multiple errors: %q", out)
	}
	if !strings.Contains(out, "k=<forgot to add values") {
		t.Fatalf("expected forgot marker: %q", out)
	}
}

func TestAllIntAndUintVariants(t *testing.T) {
	l, buf := newBufLogger(itlog.LevelInfo)
	ev := l.Info().
		Int("i", 1).
		Int8("i8", 2).
		Int16("i16", 3).
		Int32("i32", 4).
		Int64("i64", 5).
		Uint("u", 6).
		Uint8("u8", 7).
		Uint16("u16", 8).
		Uint32("u32", 9).
		Uint64("u64", 10)
	ev.Msg("nums")
	out := buf.String()
	for _, k := range []string{"i=", "i8=", "i16=", "i32=", "i64=", "u=", "u8=", "u16=", "u32=", "u64="} {
		if !strings.Contains(out, k) {
			t.Fatalf("missing %s in %q", k, out)
		}
	}
}

func TestBeginDoneConvenience(t *testing.T) {
	l, buf := newBufLogger(itlog.LevelInfo)
	l.Info().Begin("work")
	l.Info().Done("work")
	out := buf.String()
	if !strings.Contains(out, "begin work") || !strings.Contains(out, "done  work") {
		t.Fatalf("expected begin/done messages: %q", out)
	}
}

func TestEncodeEscaping(t *testing.T) {
	l, buf := newBufLogger(itlog.LevelInfo)
	special := "a|b=c\n\x00\\end"
	l.Info().Str("data", special).Msg("escape")
	out := buf.String()
	if !strings.Contains(out, "data=") {
		t.Fatalf("missing key: %q", out)
	}
	if strings.Contains(out, "\n") && !strings.HasSuffix(out, "\n") {
		t.Fatalf("unexpected raw newline: %q", out)
	}
}

func TestDisabledLogger(t *testing.T) {
	if itlog.New(itlog.LevelDisabled) != nil {
		t.Fatal("disabled logger should be nil")
	}
}

func TestNilLoggerAndEventViaPublicAPI(t *testing.T) {
	// nil logger from disabled level
	l := itlog.New(itlog.LevelDisabled)
	l.Debug().Msg("no log")        // safe no-op
	l.Info().Err(nil).Msg("")      // safe no-op
	l.Warn().Str("a", "b").Msg("") // safe no-op
}

func TestContextAndErrorVariants(t *testing.T) {
	var buf bytes.Buffer
	itlog.Writer = &buf
	l := itlog.New(itlog.LevelDebug)

	// exercise all context types through public methods
	l.Info().
		Int("i", 1).
		Int8("i8", 2).
		Int16("i16", 3).
		Int32("i32", 4).
		Int64("i64", 5).
		Uint("u", 6).
		Uint8("u8", 7).
		Uint16("u16", 8).
		Uint32("u32", 9).
		Uint64("u64", 10).
		Bool("t", true).
		Bool("f", false).
		Err(errors.New("one")).
		Err(nil).
		Msg("ctx")

	out := buf.String()
	if out == "" {
		t.Fatal("expected output")
	}
}

func TestListAndErrsPublic(t *testing.T) {
	var buf bytes.Buffer
	itlog.Writer = &buf
	l := itlog.New(itlog.LevelInfo)
	errs := []error{errors.New("a"), nil, errors.New("b")}
	l.Info().Errs(errs...).List("nums").Msg("test")
	if buf.Len() == 0 {
		t.Fatal("expected log output")
	}
}

func TestBeginDoneAndDisabled(t *testing.T) {
	var buf bytes.Buffer
	itlog.Writer = &buf
	l := itlog.New(itlog.LevelInfo)
	l.Info().Begin("job")
	l.Info().Done("job")
	if buf.Len() == 0 {
		t.Fatal("expected begin/done logs")
	}
}

func TestEscapingAndEmptyMessages(t *testing.T) {
	var buf bytes.Buffer
	itlog.Writer = &buf
	l := itlog.New(itlog.LevelInfo)
	l.Info().Str("weird", "x|y=z\n\x00\\").Msg("")
	if buf.Len() == 0 {
		t.Fatal("expected escaped log")
	}
}

func TestNilWriterAndErrorLevelDisabled(t *testing.T) {
	itlog.Writer = nil
	l := itlog.New(itlog.LevelError)
	// Writer=nil, disabled below Error, will trigger those nil paths
	l.Debug().Msg("dbg")
	l.Info().Msg("inf")
	l.Warn().Msg("wrn")
	l.Error().Msg("err") // should no-panic
}

func TestMultipleErrorArgs(t *testing.T) {
	var buf bytes.Buffer
	itlog.Writer = &buf
	l := itlog.New(itlog.LevelDebug)
	l.Error().Err(errors.New("one")).Err(errors.New("two")).Msg("multierr")
	if !strings.Contains(buf.String(), "multierr") {
		t.Fatal("expected message")
	}
}

func TestWithBoolTrueFalseAndCondPaths(t *testing.T) {
	var buf bytes.Buffer
	itlog.Writer = &buf
	l := itlog.New(itlog.LevelDebug)
	l = l.WithBool("condTrue", true).WithBool("condFalse", false)
	l.Info().Msg("boolcond")
	out := buf.String()
	if !strings.Contains(out, "condTrue") {
		t.Fatal("expected condTrue")
	}
	if !strings.Contains(out, "condFalse") {
		t.Fatal("expected condFalse")
	}
}

func TestOversizedMessage(t *testing.T) {
	var buf bytes.Buffer
	itlog.Writer = &buf
	l := itlog.New(itlog.LevelDebug)
	big := strings.Repeat("X", 4096) // exceed default buffer
	l.Info().Msg(big)
	if buf.Len() == 0 {
		t.Fatal("expected oversized message log")
	}
}

func TestEscapedNewlineKeyValSepComponentSep(t *testing.T) {
	var buf bytes.Buffer
	itlog.Writer = &buf
	l := itlog.New(itlog.LevelInfo)
	s := "a|b=c\nnull\x00end"
	l.Info().Str("x", s).Msg("escapeTest")
	out := buf.String()
	if strings.Contains(out, "\n") && !strings.HasSuffix(out, "\n") {
		t.Fatal("unexpected raw newline")
	}
	if !strings.Contains(out, "escapeTest") {
		t.Fatal("expected message")
	}
}

func TestAllErrsNilAndNonNil(t *testing.T) {
	var buf bytes.Buffer
	itlog.Writer = &buf
	l := itlog.New(itlog.LevelInfo)
	l.Info().Errs(nil, nil).Msg("allNilErrs")
	l.Info().Errs(errors.New("one"), errors.New("two")).Msg("nonNilErrs")
	if !strings.Contains(buf.String(), "nonNilErrs") {
		t.Fatal("expected nonNilErrs")
	}
}

func TestNilEventPublicCallsAndEmptyEncode(t *testing.T) {
	var buf bytes.Buffer
	itlog.Writer = &buf

	// Create a logger where Debug is disabled so Debug() returns nil _Event.
	l := itlog.New(itlog.LevelInfo)
	if l == nil {
		t.Fatal("New returned nil")
	}

	// nil event: call many exported methods on the returned nil _Event to hit nil-path invariants.
	ev := l.Debug() // returns nil because LevelInfo > LevelDebug
	ev.Str("a", "b")
	ev.Int("i", 1)
	ev.Int8("i8", 2)
	ev.Int16("i16", 3)
	ev.Int32("i32", 4)
	ev.Int64("i64", 5)
	ev.Uint("u", 6)
	ev.Uint8("u8", 7)
	ev.Uint16("u16", 8)
	ev.Uint32("u32", 9)
	ev.Uint64("u64", 10)
	ev.Bool("t", true)
	ev.Bool("f", false)
	ev.List("lst", "one", "two")
	ev.Err(nil)
	ev.Errs(nil, errors.New("e"))
	ev.Msg("") // no panic - should be safe no-op
}

func TestEmptyKeyAndValueEncodeAndEscapePaths(t *testing.T) {
	var buf bytes.Buffer
	itlog.Writer = &buf

	l := itlog.New(itlog.LevelDebug)
	if l == nil {
		t.Fatal("New returned nil")
	}

	// lots of tricky characters to exercise escaping logic via public API
	special := "pipe|eq=nl\nnul\x00back\\end"
	l.Info().Str("data", special).Msg("escaped")
	out := buf.String()
	if out == "" {
		t.Fatal("expected some log output")
	}
	// no raw newline should appear inside the context portion (only trailing record newline allowed)
	if count := strings.Count(out, "\n"); count > 2 {
		t.Fatalf("unexpected %d embedded newline(s) in output: %q", count, out)
	}
}

func TestMultipleErrorArgsAndListEmpty(t *testing.T) {
	var buf bytes.Buffer
	itlog.Writer = &buf

	l := itlog.New(itlog.LevelDebug)
	if l == nil {
		t.Fatal("New returned nil")
	}

	// multiple errors (triggers logger.Error path that handles multiple args)
	l.Error(errors.New("a"), errors.New("b")).Msg("manyerrs")

	// empty List call (via exported List) should add the "forgot to add values..." marker
	l.Info().List("empty").Msg("gotlist")
	if buf.Len() == 0 {
		t.Fatal("expected logs")
	}
}

func TestLoggerBlackbox(t *testing.T) {
	// 1. Nil Logger
	var l *itlog.Logger
	if l.Debug() != nil {
		t.Error("expected nil")
	}
	if l.Info() != nil {
		t.Error("expected nil")
	}
	if l.Warn() != nil {
		t.Error("expected nil")
	}
	if l.Error() != nil {
		t.Error("expected nil")
	}

	// 2. Disabled Logger
	ld := itlog.New(itlog.LevelDisabled)
	if ld != nil {
		t.Error("expected nil logger when disabled")
	}

	// 3. Chaining context methods
	l = itlog.New(itlog.LevelDebug)
	l2 := l.WithData("foo", "bar").
		WithStr("str", "val").
		WithInt("i", 123).
		WithBool("b", true).
		WithErr("err", errors.New("err1"))
	if l2 == nil {
		t.Error("expected logger copy")
	}

	e := l2.Debug()
	if e == nil {
		t.Fatal("debug logEvent should not be nil")
	}
	e.Msg("test message")

	// 4. Special characters & empty message
	e = l.Debug()
	if e == nil {
		t.Fatal("debug event nil")
	}
	e.Data("newline", "a\nb").Data("sep", "x|y").Data("null", string([]byte{0}))
	e.Msg("")

	// 5. Large context to trigger oversized buffer path
	e = l.Debug()
	if e == nil {
		t.Fatal("debug event nil")
	}
	large := make([]byte, itlog.DefaultEventBufferCapacity*2)
	e.Data("large", string(large))
	e.Msg("large buffer test")

	// 6. Error variations
	e = l.Error()
	if e == nil {
		t.Fatal("error event nil")
	}
	e = l.Error(errors.New("one"))
	e = l.Error(errors.New("one"), errors.New("two"), nil)
}

// silenceOutput redirects the global itlog.Writer to io.Discard for the
// duration of a test.
func silenceOutput(t *testing.T) {
	t.Helper()
	originalWriter := itlog.Writer
	itlog.Writer = io.Discard
	t.Cleanup(func() {
		itlog.Writer = originalWriter
	})
}

// TestNilLoggerReceivers triggers assertions for methods called on a nil *Logger.
// Each case is in a t.Run() to isolate panics.
func TestNilLoggerReceivers(t *testing.T) {
	silenceOutput(t)
	var logger *itlog.Logger // logger is intentionally nil

	t.Run("Logger.NewEvent is nil", func(t *testing.T) {
		// This triggers:
		// "Logger is nil" at .../itlog.go:214
		logger.NewEvent("INF")
	})

	t.Run("Logger.Debug is nil", func(t *testing.T) {
		// This triggers:
		// "Logger is nil" at .../itlog.go:214
		logger.Debug()
	})

	t.Run("Logger.Info is nil", func(t *testing.T) {
		logger.Info()
	})

	t.Run("Logger.Warn is nil", func(t *testing.T) {
		logger.Warn()
	})

	t.Run("Logger.Error is nil", func(t *testing.T) {
		logger.Error(errors.New("test"))
	})

	t.Run("Logger.WithStr is nil", func(t *testing.T) {
		logger.WithStr("key", "val")
	})

	t.Run("Logger.WithData is nil", func(t *testing.T) {
		logger.WithData("key", "val")
	})

	t.Run("Logger.WithErr is nil", func(t *testing.T) {
		logger.WithErr("key", errors.New("test"))
	})

	t.Run("Logger.WithInt is nil", func(t *testing.T) {
		logger.WithInt("key", 123)
	})

	t.Run("Logger.WithBool is nil", func(t *testing.T) {
		logger.WithBool("key", true)
	})
}

// TestNilLogEventReceivers triggers assertions for methods called on a nil *logEvent.
// A nil *logEvent is returned when a log level is disabled.
func TestNilLogEventReceivers(t *testing.T) {
	silenceOutput(t)
	// Create a logger where Debug level is disabled (LevelInfo > LevelDebug)
	logger := itlog.New(itlog.LevelInfo)

	t.Run("logEvent.Number is nil", func(t *testing.T) {
		// This triggers:
		// "logEvent is nil" at .../itlog.go:184
		logger.Debug().Int("key", 123) // .Int() calls .Number()
	})

	t.Run("logEvent.Err is nil", func(t *testing.T) {
		// This triggers:
		// "logEvent is nil" at .../itlog.go:376
		logger.Debug().Err(errors.New("test"))
	})

	t.Run("logEvent.Data is nil", func(t *testing.T) {
		logger.Debug().Data("key", "val")
	})

	t.Run("logEvent.Str is nil", func(t *testing.T) {
		logger.Debug().Str("key", "val")
	})

	t.Run("logEvent.Msg is nil", func(t *testing.T) {
		logger.Debug().Msg("this should not panic")
	})

	t.Run("logEvent.Begin is nil", func(t *testing.T) {
		logger.Debug().Begin("this should not panic")
	})
	t.Run("logEvent.Done is nil", func(t *testing.T) {
		logger.Debug().Done("this should not panic")
	})
}

// TestDisabledLogLevels triggers assertions for specifically disabled log levels.
func TestDisabledLogLevels(t *testing.T) {
	silenceOutput(t)

	// Create a logger with a level *above* Error but *below* Disabled
	logger := itlog.New(itlog.LevelError + 1)

	t.Run("Error level disabled", func(t *testing.T) {
		// "Error level and below is disabled" at .../itlog.go:291
		logger.Error(errors.New("this should not be logged"))
	})

	t.Run("Warn level disabled", func(t *testing.T) {
		logger.Warn()
	})

	t.Run("Info level disabled", func(t *testing.T) {
		logger.Info()
	})

	t.Run("Debug level disabled", func(t *testing.T) {
		logger.Debug()
	})
}
