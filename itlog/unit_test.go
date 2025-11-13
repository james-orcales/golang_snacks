package itlog_test

import (
	"bytes"
	"errors"
	"os"
	"testing"
	"time"

	"golang_snacks/invariant"
	"golang_snacks/itlog"
	"golang_snacks/snap"
)

var LogOutputBuffer = &bytes.Buffer{}

func TestMain(m *testing.M) {
	itlog.TickCallback = func() time.Time {
		return time.Date(2000, 2, 0, 23, 59, 59, 0, time.UTC)
	}
	LogOutputBuffer.Grow(1024 * 1024 * 1)

	invariant.RegisterPackagesForAnalysis()
	code := m.Run()
	if code == 0 {
		invariant.AnalyzeAssertionFrequency()
	}
	os.Exit(code)
}

func check(t *testing.T, actual string, snapshot snap.Snapshot) {
	t.Helper()
	if !snapshot.Diff(actual) {
		t.Fatal("Snapshot mismatch")
	}
}

func TestSanityCheck(t *testing.T) {
	LogOutputBuffer.Reset()
	lgr := itlog.New(LogOutputBuffer, itlog.LevelDebug)
	lgr.Error(errors.New("カワキヲアメク"), errors.New(""))

	lgr = lgr.WithStr("", "test\n\x00test").
		WithBool("bool.bar_baz", true).
		WithErr("error.bar_baz", errors.New("test\n\x00test")).
		WithBool("bool.bar_baz", false).
		WithUint64("uint64.bar_baz", 1<<64-1).
		WithInt8("int8.bar_baz", -1<<7)
	lgr.Debug().
		Str("str.ev", "test\n\x00test").
		Uint64("uint64.ev", uint64(1<<64-1)).
		Int("int.ev", int(-1<<63)).
		Int8("int8.ev", int8(-1<<7)).
		Int16("int16.ev", int16(-1<<15)).
		Int32("int32.ev", int32(-1<<31)).
		Int64("int64.ev2", int64(-1<<63)).
		Uint("uint.ev", uint(1<<64-1)).
		Uint8("uint8.ev", uint8(1<<8-1)).
		Uint16("uint16.ev", uint16(1<<16-1)).
		Uint32("uint32.ev", uint32(1<<32-1)).
		Bool("bool.ev", true).
		Bool("bool.ev", false).
		Err(errors.New("err\n\x00err")).
		Msg(" test\n\x00test")

	check(t, LogOutputBuffer.String(), snap.New(`2000-01-31T23:59:59Z|DBG| test  test                                                                     |__EMPTY__="test\n\0test"|bool.bar_baz=true|error.bar_baz="test\n\0test"|bool.bar_baz=false|uint64.bar_baz=18446744073709551615|int8.bar_baz=-128|str.ev="test\n\0test"|uint64.ev=18446744073709551615|int.ev=-9223372036854775808|int8.ev=-128|int16.ev=-32768|int32.ev=-2147483648|int64.ev2=-9223372036854775808|uint.ev=18446744073709551615|uint8.ev=255|uint16.ev=65535|uint32.ev=4294967295|bool.ev=true|bool.ev=false|error="err\n\0err"|
`))
}

func TestMessage(t *testing.T) {
	LogOutputBuffer.Reset()
	lgr := itlog.New(LogOutputBuffer, itlog.LevelDebug)
	inputs := []string{
		"",
		"\n\n",
		"\x00\x00",
		"\x00\n\x00\n",
		"\x00\nline\nbreak\x00",
		"a\x00b",
		"x\n\x00y",
		"this message fills the buffer EXACTLY..........................................\n",
		"very long message that exceeds message capacityvery long message that exceeds message capacityvery long message that exceeds message capacityvery long message that exceeds message capacity",
		` 未熟 無ジョウ されど 美しくあれ
No destiny ふさわしく無い
こんなんじゃきっと物足りない`,
	}

	for _, input := range inputs {
		lgr.Warn().Msg(input)
	}
	check(t, LogOutputBuffer.String(), snap.New(`2000-01-31T23:59:59Z|WRN|__EMPTY__                                                                       |
2000-01-31T23:59:59Z|WRN|                                                                                |
2000-01-31T23:59:59Z|WRN|                                                                                |
2000-01-31T23:59:59Z|WRN|                                                                                |
2000-01-31T23:59:59Z|WRN|  line break                                                                    |
2000-01-31T23:59:59Z|WRN|a b                                                                             |
2000-01-31T23:59:59Z|WRN|x  y                                                                            |
2000-01-31T23:59:59Z|WRN|this message fills the buffer EXACTLY.......................................... |
2000-01-31T23:59:59Z|WRN|very long message that exceeds message capacityvery long message that exceeds me|
2000-01-31T23:59:59Z|WRN| 未熟 無ジョウ されど 美しくあれ No destiny ふさわしく無い |
`))
}

func TestNilLogger(t *testing.T) {
	LogOutputBuffer.Reset()
	lgr := itlog.New(LogOutputBuffer, itlog.LevelDisabled).
		WithErr("", nil).
		WithStr("", "").
		WithBool("", true).
		WithInt8("", -1<<7).
		WithUint64("", 1<<64-1)
	lgr.Debug().Msg("")
	lgr.Info().Msg("")
	lgr.Warn().Msg("")
	lgr.Error().
		Str("", "").
		Uint64("", uint64(1<<64-1)).
		Int("", int(-1<<63)).
		Int8("", int8(-1<<7)).
		Int16("", int16(-1<<15)).
		Int32("", int32(-1<<31)).
		Int64("", int64(-1<<63)).
		Uint("", uint(1<<64-1)).
		Uint8("", uint8(1<<8-1)).
		Uint16("", uint16(1<<16-1)).
		Uint32("", uint32(1<<32-1)).
		Bool("", true).
		Err(errors.New("")).
		Errs(errors.New("")).
		Msg("")
	lgr.Info().Begin("")
	lgr.Info().Done("")
	check(t, LogOutputBuffer.String(), snap.New(``))
}

func TestLevelThresholds(t *testing.T) {
	LogOutputBuffer.Reset()
	lgr := itlog.New(LogOutputBuffer, itlog.LevelDisabled-1).
		WithErr("", nil).
		WithBool("", true).
		WithInt8("", -1<<7).
		WithUint64("", 1<<64-1)
	lgr.Debug().Msg("")
	lgr.Info().Msg("")
	lgr.Warn().Msg("")
	lgr.Error().
		Str("", "").
		Uint64("", uint64(1<<64-1)).
		Int("", int(-1<<63)).
		Int8("", int8(-1<<7)).
		Int16("", int16(-1<<15)).
		Int32("", int32(-1<<31)).
		Int64("", int64(-1<<63)).
		Uint("", uint(1<<64-1)).
		Uint8("", uint8(1<<8-1)).
		Uint16("", uint16(1<<16-1)).
		Uint32("", uint32(1<<32-1)).
		Bool("", true).
		Err(errors.New("")).
		Msg("")
	lgr.Info().Begin("")
	lgr.Info().Done("")
	check(t, LogOutputBuffer.String(), snap.New(``))
}

func TestEmpty(t *testing.T) {
	LogOutputBuffer.Reset()
	lgr := itlog.New(LogOutputBuffer, itlog.LevelError).
		WithStr("", "").
		WithErr("", nil).
		WithBool("", true).
		WithInt8("", 1<<7-1).
		WithUint64("", 1<<64-1)
	lgr.Error().
		Str("", "").
		Uint64("", uint64(1<<64-1)).
		Int("", int(-1<<63)).
		Int8("", int8(-1<<7)).
		Int16("", int16(-1<<15)).
		Int32("", int32(-1<<31)).
		Int64("", int64(-1<<63)).
		Uint("", uint(1<<64-1)).
		Uint8("", uint8(1<<8-1)).
		Uint16("", uint16(1<<16-1)).
		Uint32("", uint32(1<<32-1)).
		Bool("", true).
		Err(nil).
		Errs(nil, nil, nil, nil).
		Msg("")
	check(t, LogOutputBuffer.String(), snap.New(`2000-01-31T23:59:59Z|ERR|__EMPTY__                                                                       |__EMPTY__="__EMPTY__"|__EMPTY__=true|__EMPTY__=127|__EMPTY__=18446744073709551615|__EMPTY__="__EMPTY__"|__EMPTY__=18446744073709551615|__EMPTY__=-9223372036854775808|__EMPTY__=-128|__EMPTY__=-32768|__EMPTY__=-2147483648|__EMPTY__=-9223372036854775808|__EMPTY__=18446744073709551615|__EMPTY__=255|__EMPTY__=65535|__EMPTY__=4294967295|__EMPTY__=true|
`))
}

func TestNilWriter(t *testing.T) {
	lgr := itlog.New(nil, itlog.LevelInfo)
	if lgr != nil {
		t.Fatal("Initialized logger is not nil when writer is nil")
	}
}

func TestErrorConvenience(t *testing.T) {
	LogOutputBuffer.Reset()
	lgr := itlog.New(LogOutputBuffer, itlog.LevelError)
	lgr.Error().Msg("")
	lgr.Error(nil).Msg("")
	lgr.Error(nil, nil, nil).Msg("")
	lgr.Error(errors.New("foo")).Msg("")
	lgr.Error(nil, errors.New("bar"), nil).Msg("")
	lgr.Error(errors.New("foo"), errors.New("bar"), errors.New("baz")).Msg("")

	check(t, LogOutputBuffer.String(), snap.New(`2000-01-31T23:59:59Z|ERR|__EMPTY__                                                                       |
2000-01-31T23:59:59Z|ERR|__EMPTY__                                                                       |
2000-01-31T23:59:59Z|ERR|__EMPTY__                                                                       |
2000-01-31T23:59:59Z|ERR|__EMPTY__                                                                       |error="foo"|
2000-01-31T23:59:59Z|ERR|__EMPTY__                                                                       |error="bar"|
2000-01-31T23:59:59Z|ERR|__EMPTY__                                                                       |error="foo"|error="bar"|error="baz"|
`))
}

func TestBeginDone(t *testing.T) {
	LogOutputBuffer.Reset()
	lgr := itlog.New(LogOutputBuffer, itlog.LevelInfo)
	lgr.Info().Begin("validating cache")
	lgr.Info().Done("validating cache")
	check(t, LogOutputBuffer.String(), snap.New(`2000-01-31T23:59:59Z|INF|begin validating cache                                                          |
2000-01-31T23:59:59Z|INF|done  validating cache                                                          |
`))
}
