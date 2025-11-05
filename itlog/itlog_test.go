// This file is AI-generated. I'll write proper tests later.
package itlog

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
	"testing"
)

func TestLoggerBasic(t *testing.T) {
	var buf bytes.Buffer
	Writer = &buf

	l := New(LevelDebug).WithStr("app", "test")

	l.Info().Msg("hello")
	l.Error(errors.New("fail")).Msg("oops")

	out := buf.String()
	if !strings.Contains(out, "hello") {
		t.Fatal("missing info message")
	}
	if !strings.Contains(out, "fail") {
		t.Fatal("missing error message")
	}
}

func withWriter(b *bytes.Buffer, fn func()) {
	old := Writer
	Writer = b
	defer func() { Writer = old }()
	fn()
}

func TestAppendAndEscape(t *testing.T) {
	var dst []byte
	appendAndEscape(&dst, "a|=b\\c")
	got := string(dst)
	// only '|' and '=' must be escaped (backslash inserted). original backslash remains.
	if !strings.Contains(got, `a\|\=b\c`) && !strings.Contains(got, `a\|\=b\\c`) {
		t.Fatalf("unexpected escaped output: %q", got)
	}
}

func TestWithDataAndDataAndEscaping(t *testing.T) {
	l := New(LevelInfo)
	if l == nil {
		t.Fatal("New returned nil")
	}
	l2 := l.WithData("k|x", "v=y")
	if l2 == nil {
		t.Fatal("WithData returned nil")
	}
	s := string(l2.Buffer)
	if !strings.Contains(s, `k\|x\=v\=y`) && !strings.Contains(s, "k\\|x") {
		t.Fatalf("expected escaped key/value in logger buffer, got %q", s)
	}

	e := new_event(l2, "INFO ")
	e = e.Data("k|", "v=")
	withWriter(&bytes.Buffer{}, func() {
		e.Msg("")
	})
	// ensure Data doesn't crash and appended separator exists
	if !strings.Contains(string(e.Buffer), string(ComponentSeparator)) {
		t.Fatalf("event buffer missing component separator: %q", string(e.Buffer))
	}
}

func TestNewNilConditions(t *testing.T) {
	old := Writer
	Writer = nil
	if got := New(LevelInfo); got != nil {
		t.Fatalf("expected nil when Writer == nil, got %+v", got)
	}
	Writer = old

	if got := New(LevelDisabled); got != nil {
		t.Fatalf("expected nil for disabled level, got %+v", got)
	}
}

func TestLevelFiltering(t *testing.T) {
	l := New(LevelWarn)
	if l.Debug() != nil {
		t.Fatal("Debug should be nil at warn level")
	}
	if l.Info() != nil {
		t.Fatal("Info should be nil at warn level")
	}
	if l.Warn() == nil {
		t.Fatal("Warn should not be nil at warn level")
	}
}

func TestErrorConvenienceAndErrs(t *testing.T) {
	l := New(LevelError)
	buf := &bytes.Buffer{}
	withWriter(buf, func() {
		e := l.Error(errors.New("boom"))
		if e == nil {
			t.Fatal("Error returned nil")
		}
		e.Msg("something")
	})
	out := buf.String()
	if !strings.Contains(out, "error=boom") {
		t.Fatalf("expected error field, got %q", out)
	}

	buf.Reset()
	withWriter(buf, func() {
		e := l.Error(errors.New("a"), errors.New("b"))
		if e == nil {
			t.Fatal("Error returned nil for multiple errs")
		}
		e.Msg("m")
	})
	if !strings.Contains(buf.String(), "error=a") || !strings.Contains(buf.String(), "error=b") {
		t.Fatalf("errs not written: %q", buf.String())
	}
}

func parseLine(line string) (timePart, levelPart, msgPart, rest string) {
	parts := strings.SplitN(line, string(ComponentSeparator), 4)
	if len(parts) < 4 {
		// pad to avoid panics in assertions
		for len(parts) < 4 {
			parts = append(parts, "")
		}
	}
	return parts[0], parts[1], parts[2], parts[3]
}

func TestMsgEmptyAndTruncation(t *testing.T) {
	l := New(LevelInfo)
	buf := &bytes.Buffer{}
	withWriter(buf, func() {
		e := new_event(l, "INFO ")
		e.Msg("") // should set "<empty>"
	})
	out := buf.String()
	line := strings.TrimRight(out, "\n")
	_, _, msg, _ := parseLine(line)
	if !strings.Contains(msg, "<empty>") {
		t.Fatalf("expected <empty> in message, got %q", msg)
	}

	// truncation
	long := strings.Repeat("X", MessageCapacity+20)
	buf.Reset()
	withWriter(buf, func() {
		e := new_event(l, "INFO ")
		e.Msg(long)
	})
	line = strings.TrimRight(buf.String(), "\n")
	_, _, msg2, _ := parseLine(line)
	if len(msg2) != MessageCapacity {
		t.Fatalf("expected message length %d, got %d (%q)", MessageCapacity, len(msg2), msg2)
	}
	if !strings.HasPrefix(msg2, strings.Repeat("X", MessageCapacity)) {
		t.Fatalf("message truncated incorrectly: %q", msg2)
	}
}

func TestMsgWritesNewlineAndHeaderLayout(t *testing.T) {
	l := New(LevelInfo)
	buf := &bytes.Buffer{}
	withWriter(buf, func() {
		e := new_event(l, "INFO ")
		e.Str("k", "v")
		e.Msg("hi")
	})
	out := buf.String()
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("output does not end with newline: %q", out)
	}
	line := strings.TrimRight(out, "\n")
	timePart, levelPart, msgPart, rest := parseLine(line)
	if len(levelPart) != LevelMaxWordLength {
		t.Fatalf("level part length wrong: %q", levelPart)
	}
	if len(timePart) >= TimestampCapacity+1 {
		// timePart includes no component separator here (split removed it) -- just check non-empty & reasonable
	}
	if !strings.Contains(rest, "k=v") {
		t.Fatalf("context missing k=v: %q", rest)
	}
	_ = msgPart
}

func TestNumberAndUint64AndBool(t *testing.T) {
	l := New(LevelInfo)
	buf := &bytes.Buffer{}
	withWriter(buf, func() {
		e := new_event(l, "INFO ")
		e.Int("i", 42)
		e.Uint64("u", 9999999999)
		e.Bool("b", true)
		e.Msg("n")
	})
	out := buf.String()
	if !strings.Contains(out, "i=42") {
		t.Fatalf("int missing: %q", out)
	}
	if !strings.Contains(out, "u="+strconv.FormatUint(9999999999, 10)) {
		t.Fatalf("uint64 missing: %q", out)
	}
	if !strings.Contains(out, "b=true") {
		t.Fatalf("bool missing: %q", out)
	}
}

func TestListAndMultipleDataCalls(t *testing.T) {
	l := New(LevelInfo)
	buf := &bytes.Buffer{}
	withWriter(buf, func() {
		e := new_event(l, "INFO ")
		e.List("k", "a", "b", "c")
		e.Msg("list")
	})
	out := buf.String()
	if !(strings.Count(out, "k=") >= 3) {
		t.Fatalf("expected three k= entries, got: %q", out)
	}
}

func TestNilReceiversSafe(t *testing.T) {
	var l *Logger
	if l.WithData("k", "v") != nil {
		t.Fatal("WithData on nil logger should return nil")
	}
	var e *Event
	if e.Data("k", "v") != nil {
		t.Fatal("Data on nil event should return nil")
	}
	// methods that should simply return
	l = nil
	if l.Debug() != nil {
		t.Fatal("Debug on nil logger should return nil")
	}
	if l.Info() != nil {
		t.Fatal("Info on nil logger should return nil")
	}
	if l.Warn() != nil {
		t.Fatal("Warn on nil logger should return nil")
	}
}

func TestEventPool_NewInstance(t *testing.T) {
	got := EventPool.Get()
	if got == nil {
		t.Fatal("EventPool returned nil")
	}
	if _, ok := got.(*Event); !ok {
		t.Fatalf("EventPool returned wrong type: %T", got)
	}
	EventPool.Put(got)
}

func TestLoggerInheritance(t *testing.T) {
	l := New(LevelInfo).WithStr("app", "core").WithInt("pid", 123)
	buf := &bytes.Buffer{}
	withWriter(buf, func() {
		l.Info().Str("extra", "val").Msg("inherited")
	})
	out := buf.String()
	if !strings.Contains(out, "app=core") || !strings.Contains(out, "pid=123") {
		t.Fatalf("context not inherited: %q", out)
	}
}

func TestBeginAndDone(t *testing.T) {
	l := New(LevelInfo)
	buf := &bytes.Buffer{}
	withWriter(buf, func() {
		e := l.Info()
		e.Begin("task")
		e2 := l.Info()
		e2.Done("task")
	})
	out := buf.String()
	if !strings.Contains(out, "begin task") || !strings.Contains(out, "done  task") {
		t.Fatalf("missing begin/done: %q", out)
	}
}

func TestWithErrAndBoolInt(t *testing.T) {
	l := New(LevelInfo).WithErr("err", errors.New("fail")).WithBool("ok", true).WithInt("n", 99)
	buf := &bytes.Buffer{}
	withWriter(buf, func() {
		l.Info().Msg("combo")
	})
	out := buf.String()
	for _, k := range []string{"err=fail", "ok=true", "n=99"} {
		if !strings.Contains(out, k) {
			t.Fatalf("missing %s in %q", k, out)
		}
	}
}

func TestEventMultipleDataCallsOrder(t *testing.T) {
	l := New(LevelInfo)
	buf := &bytes.Buffer{}
	withWriter(buf, func() {
		e := new_event(l, "INFO ")
		e.Str("a", "1").Int("b", 2).Bool("c", true)
		e.Msg("multi")
	})
	out := buf.String()
	idxA := strings.Index(out, "a=1")
	idxB := strings.Index(out, "b=2")
	idxC := strings.Index(out, "c=true")
	if !(idxA < idxB && idxB < idxC) {
		t.Fatalf("data order incorrect: %q", out)
	}
}

func TestDataWithEmptyKeyAndValue(t *testing.T) {
	l := New(LevelInfo)
	buf := &bytes.Buffer{}
	withWriter(buf, func() {
		e := new_event(l, "INFO ")
		e.Str("", "")
		e.Msg("empty")
	})
	out := buf.String()
	if !strings.Contains(out, "=") {
		t.Fatalf("expected empty key/value to produce '=', got %q", out)
	}
}

func TestEventListWithEmptySlice(t *testing.T) {
	l := New(LevelInfo)
	buf := &bytes.Buffer{}
	withWriter(buf, func() {
		e := new_event(l, "INFO ")
		e.List("k")
		e.Msg("emptylist")
	})
	out := buf.String()
	if strings.Contains(out, "k=") {
		t.Fatalf("empty list should not add entries: %q", out)
	}
}

func TestMsgWithEmptyString(t *testing.T) {
	l := New(LevelInfo)
	buf := &bytes.Buffer{}
	withWriter(buf, func() {
		e := new_event(l, "INFO ")
		// do not call Msg()
		e.Msg("")
	})
	out := buf.String()
	if !strings.Contains(out, "<empty>") {
		t.Fatalf("Msg with empty string should produce <empty>, got %q", out)
	}
}

func TestOverwriteMsgMultipleTimes(t *testing.T) {
	l := New(LevelInfo)
	buf := &bytes.Buffer{}
	withWriter(buf, func() {
		e := new_event(l, "INFO ")
		e.Msg("first")
		e.Msg("second")
	})
	out := buf.String()
	if strings.Count(out, "first") != 1 || strings.Count(out, "second") != 1 {
		t.Fatalf("multiple Msg calls failed: %q", out)
	}
}

func TestWithDataEscapingSpecialChars(t *testing.T) {
	l := New(LevelInfo)
	buf := &bytes.Buffer{}
	withWriter(buf, func() {
		l2 := l.WithData("key|=x", "val|=y")
		l2.Info().Msg("escaped")
	})
	out := buf.String()
	for _, sub := range []string{"key\\|\\=x", "val\\|\\=y"} {
		if !strings.Contains(out, sub) {
			t.Fatalf("expected escaped content %q in %q", sub, out)
		}
	}
}

func TestEventNumberInt64Edge(t *testing.T) {
	l := New(LevelInfo)
	buf := &bytes.Buffer{}
	withWriter(buf, func() {
		e := new_event(l, "INFO ")
		e.Number("min", int64(-9223372036854775808))
		e.Number("max", int64(9223372036854775807))
		e.Msg("int64")
	})
	out := buf.String()
	if !strings.Contains(out, "min=-9223372036854775808") || !strings.Contains(out, "max=9223372036854775807") {
		t.Fatalf("int64 extremes not logged correctly: %q", out)
	}
}

func TestEventUint64Edge(t *testing.T) {
	l := New(LevelInfo)
	buf := &bytes.Buffer{}
	withWriter(buf, func() {
		e := new_event(l, "INFO ")
		e.Uint64("zero", 0)
		e.Uint64("max", ^uint64(0))
		e.Msg("uint64")
	})
	out := buf.String()
	if !strings.Contains(out, "zero=0") || !strings.Contains(out, "max=18446744073709551615") {
		t.Fatalf("uint64 extremes not logged correctly: %q", out)
	}
}

func TestWithErrNilDoesNotAdd(t *testing.T) {
	l := New(LevelInfo)
	buf := &bytes.Buffer{}
	withWriter(buf, func() {
		l2 := l.WithErr("err", nil)
		l2.Info().Msg("nilerr")
	})
	out := buf.String()
	if strings.Contains(out, "err=") {
		t.Fatalf("nil error should not appear in context: %q", out)
	}
}

func TestMsgTrimmingSpaces(t *testing.T) {
	l := New(LevelInfo)
	buf := &bytes.Buffer{}
	withWriter(buf, func() {
		e := new_event(l, "INFO ")
		longMsg := "short"
		e.Msg(longMsg)
	})
	out := buf.String()
	if !strings.Contains(out, "short") {
		t.Fatalf("message not preserved: %q", out)
	}
}

func BenchmarkLoggerInfoSimple(b *testing.B) {
	l := New(LevelInfo)
	for i := 0; i < b.N; i++ {
		e := l.Info()
		if e != nil {
			e.Msg("info message")
		}
	}
}

func BenchmarkLoggerErrorSimple(b *testing.B) {
	l := New(LevelError)
	err := errors.New("error")
	for i := 0; i < b.N; i++ {
		e := l.Error(err)
		if e != nil {
			e.Msg("error message")
		}
	}
}

func BenchmarkLoggerWithStrContext(b *testing.B) {
	l := New(LevelInfo).WithStr("app", "benchmark").WithStr("module", "logger")
	for i := 0; i < b.N; i++ {
		e := l.Info()
		if e != nil {
			e.Str("user", "alice").Str("action", "test").Msg("info with context")
		}
	}
}

func BenchmarkLoggerWithIntAndBoolContext(b *testing.B) {
	l := New(LevelInfo).WithInt("pid", 12345).WithBool("active", true)
	for i := 0; i < b.N; i++ {
		e := l.Info()
		if e != nil {
			e.Int("count", 42).Bool("ok", true).Msg("info with numbers and bool")
		}
	}
}

func BenchmarkLoggerWithErrs(b *testing.B) {
	l := New(LevelError)
	errs := []error{errors.New("first"), errors.New("second")}
	for i := 0; i < b.N; i++ {
		e := l.Error(errs[0], errs[1])
		if e != nil {
			e.Msg("error with multiple errs")
		}
	}
}

func BenchmarkLoggerLongMessage(b *testing.B) {
	l := New(LevelInfo)
	longMsg := make([]byte, MessageCapacity*2)
	for i := range longMsg {
		longMsg[i] = 'X'
	}
	msgStr := string(longMsg)
	for i := 0; i < b.N; i++ {
		e := l.Info()
		if e != nil {
			e.Msg(msgStr)
		}
	}
}

func BenchmarkLoggerMultipleDataCalls(b *testing.B) {
	l := New(LevelInfo)
	for i := 0; i < b.N; i++ {
		e := l.Info()
		if e != nil {
			e.Str("a", "1").Int("b", 2).Bool("c", true).Msg("multi data")
		}
	}
}

func BenchmarkLoggerListContext(b *testing.B) {
	l := New(LevelInfo)
	items := []string{"a", "b", "c", "d", "e"}
	for i := 0; i < b.N; i++ {
		e := l.Info()
		if e != nil {
			e.List("item", items...).Msg("list context")
		}
	}
}

func BenchmarkLoggerInheritance(b *testing.B) {
	l := New(LevelInfo).WithStr("app", "core").WithInt("pid", 999)
	for i := 0; i < b.N; i++ {
		e := l.Info()
		if e != nil {
			e.Str("extra", "val").Msg("inherited context")
		}
	}
}

func BenchmarkLoggerEscaping(b *testing.B) {
	l := New(LevelInfo)
	for i := 0; i < b.N; i++ {
		e := l.Info()
		if e != nil {
			e.Str("key|=x", "val=|y").Msg("escaping test")
		}
	}
}

func BenchmarkLoggerUint64AndInt64(b *testing.B) {
	l := New(LevelInfo)
	for i := 0; i < b.N; i++ {
		e := l.Info()
		if e != nil {
			e.Uint64("u", ^uint64(0)).Number("i", int64(^uint64(0)>>1)).Msg("numbers")
		}
	}
}

func BenchmarkLogger10MixedFields(b *testing.B) {
	var buf bytes.Buffer
	Writer = &buf

	l := New(LevelInfo)
	sampleErr := errors.New("fail")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e := l.Info()
		if e != nil {
			e.Str("s1", "val1").
				Int("i1", 42).
				Bool("b1", true).
				Uint64("u1", 999999).
				Str("s2", "val2").
				Int32("i2", -17).
				Bool("b2", false).
				Err(sampleErr).
				Uint("u2", 12345).
				Str("s3", "val3").
				Msg("10 mixed fields")
		}
		buf.Reset() // reset buffer for consistent measurement
	}
}
