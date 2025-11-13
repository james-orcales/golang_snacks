// This file is AI-generated.
package itlog_test

import (
	"bytes"
	"errors"
	"testing"

	"golang_snacks/itlog"
)

func BenchmarkLoggerInfoSimple(b *testing.B) {
	l := itlog.New(itlog.LevelInfo)
	for i := 0; i < b.N; i++ {
		e := l.Info()
		if e != nil {
			e.Msg("info message")
		}
	}
}

func BenchmarkLoggerErrorSimple(b *testing.B) {
	l := itlog.New(itlog.LevelError)
	err := errors.New("error")
	for i := 0; i < b.N; i++ {
		e := l.Error(err)
		if e != nil {
			e.Msg("error message")
		}
	}
}

func BenchmarkLoggerWithStrContext(b *testing.B) {
	l := itlog.New(itlog.LevelInfo).WithStr("app", "benchmark").WithStr("module", "logger")
	for i := 0; i < b.N; i++ {
		e := l.Info()
		if e != nil {
			e.Str("user", "alice").Str("action", "test").Msg("info with context")
		}
	}
}

func BenchmarkLoggerWithIntAndBoolContext(b *testing.B) {
	l := itlog.New(itlog.LevelInfo).WithInt("pid", 12345).WithBool("active", true)
	for i := 0; i < b.N; i++ {
		e := l.Info()
		if e != nil {
			e.Int("count", 42).Bool("ok", true).Msg("info with numbers and bool")
		}
	}
}

func BenchmarkLoggerWithErrs(b *testing.B) {
	l := itlog.New(itlog.LevelError)
	errs := []error{errors.New("first"), errors.New("second")}
	for i := 0; i < b.N; i++ {
		e := l.Error(errs[0], errs[1])
		if e != nil {
			e.Msg("error with multiple errs")
		}
	}
}

func BenchmarkLoggerLongMessage(b *testing.B) {
	l := itlog.New(itlog.LevelInfo)
	longMsg := make([]byte, itlog.MessageCapacity*2)
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
	l := itlog.New(itlog.LevelInfo)
	for i := 0; i < b.N; i++ {
		e := l.Info()
		if e != nil {
			e.Str("a", "1").Int("b", 2).Bool("c", true).Msg("multi data")
		}
	}
}

func BenchmarkLoggerListContext(b *testing.B) {
	l := itlog.New(itlog.LevelInfo)
	items := []string{"a", "b", "c", "d", "e"}
	for i := 0; i < b.N; i++ {
		e := l.Info()
		if e != nil {
			e.List("item", items...).Msg("list context")
		}
	}
}

func BenchmarkLoggerInheritance(b *testing.B) {
	l := itlog.New(itlog.LevelInfo).WithStr("app", "core").WithInt("pid", 999)
	for i := 0; i < b.N; i++ {
		e := l.Info()
		if e != nil {
			e.Str("extra", "val").Msg("inherited context")
		}
	}
}

func BenchmarkLoggerEscaping(b *testing.B) {
	l := itlog.New(itlog.LevelInfo)
	for i := 0; i < b.N; i++ {
		e := l.Info()
		if e != nil {
			e.Str("key|=x", "val=|y").Msg("escaping test")
		}
	}
}

func BenchmarkLoggerUint64AndInt64(b *testing.B) {
	l := itlog.New(itlog.LevelInfo)
	for i := 0; i < b.N; i++ {
		e := l.Info()
		if e != nil {
			e.Uint64("u", ^uint64(0)).Number("i", int64(^uint64(0)>>1)).Msg("numbers")
		}
	}
}

func BenchmarkLogger10MixedFields(b *testing.B) {
	var buf bytes.Buffer
	itlog.Writer = &buf

	l := itlog.New(itlog.LevelInfo)
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
