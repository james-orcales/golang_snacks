package itlog_test

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/james-orcales/golang_snacks/invariant"
	"github.com/james-orcales/golang_snacks/itlog"
	"github.com/james-orcales/golang_snacks/snap"
)

var (
	StdoutBuffer = &bytes.Buffer{}
	StderrBuffer = &bytes.Buffer{}
)

func TestMain(m *testing.M) {
	itlog.TickCallback = func() time.Time {
		return time.Date(2000, 2, 0, 23, 59, 59, 0, time.UTC)
	}
	StdoutBuffer.Grow(1024 * 512)
	StderrBuffer.Grow(1024 * 512)

	invariant.AssertionFailureHook = func(msg string) {
		fmt.Fprintln(StderrBuffer, msg)
	}
	invariant.RunTestMain(m)
}

func check(t *testing.T, snapshot snap.Snapshot) {
	t.Helper()
	defer StdoutBuffer.Reset()
	defer StderrBuffer.Reset()
	stdout := StdoutBuffer.String()
	stderr := StderrBuffer.String()

	actual := fmt.Sprintf("Stdout:\n%s\nStderr:\n%s", stdout, stderr)
	if !snapshot.IsEqual(actual) {
		t.Fatal("Snapshot mismatch")
	}
}

func TestSanityCheck(t *testing.T) {
	lgr := itlog.New(StdoutBuffer, itlog.LevelDebug)
	lgr.Error(errors.New("カワキヲアメク"), errors.New(""))

	lgr = lgr.WithStr("", "test\n\x00test").
		WithData(nil, nil).
		WithBool("bool.bar_baz", true).
		WithErr("error.bar_baz", errors.New("test\n\x00test")).
		WithBool("bool.bar_baz", false).
		WithUint64("uint64.bar_baz", 1<<64-1).
		WithInt8("int8.bar_baz", -1<<7)
	lgr.Debug().
		Data(nil, nil).
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

	check(t, snap.Init(`Stdout:
2000-01-31T23:59:59Z|DBG| test  test                                                                     |__EMPTY__="test\n\0test"|__EMPTY__=__EMPTY__|bool.bar_baz=true|error.bar_baz="test\n\0test"|bool.bar_baz=false|uint64.bar_baz=18446744073709551615|int8.bar_baz=-128|__EMPTY__=__EMPTY__|str.ev="test\n\0test"|uint64.ev=18446744073709551615|int.ev=-9223372036854775808|int8.ev=-128|int16.ev=-32768|int32.ev=-2147483648|int64.ev2=-9223372036854775808|uint.ev=18446744073709551615|uint8.ev=255|uint16.ev=65535|uint32.ev=4294967295|bool.ev=true|bool.ev=false|error="err\n\0err"|

Stderr:
`))
}

func TestMessage(t *testing.T) {
	lgr := itlog.New(StdoutBuffer, itlog.LevelDebug)
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
	check(t, snap.Init(`Stdout:
2000-01-31T23:59:59Z|WRN|                                                                                |
2000-01-31T23:59:59Z|WRN|                                                                                |
2000-01-31T23:59:59Z|WRN|                                                                                |
2000-01-31T23:59:59Z|WRN|                                                                                |
2000-01-31T23:59:59Z|WRN|  line break                                                                    |
2000-01-31T23:59:59Z|WRN|a b                                                                             |
2000-01-31T23:59:59Z|WRN|x  y                                                                            |
2000-01-31T23:59:59Z|WRN|this message fills the buffer EXACTLY.......................................... |
2000-01-31T23:59:59Z|WRN|very long message that exceeds message capacityvery long message that exceeds me|
2000-01-31T23:59:59Z|WRN| 未熟 無ジョウ されど 美しくあれ No destiny ふさわしく無い |

Stderr:
`))
}

func TestEscaping(t *testing.T) {
	lgr := itlog.New(StdoutBuffer, itlog.LevelInfo)
	lgr.Info().Str("raw_chars", "\\\n\"\x00").Msg("")
	lgr.Info().Str("escaped_chars", `\n\"\0\\`).Msg("")
	lgr.Info().Str("mixed_chars", "\\\"|\n\\\n\\n\x00\\0\"|").Msg("")
	check(t, snap.Init(`Stdout:
2000-01-31T23:59:59Z|INF|                                                                                |raw_chars="\\\n\"\0"|
2000-01-31T23:59:59Z|INF|                                                                                |escaped_chars="\\n\\\"\\0\\\\"|
2000-01-31T23:59:59Z|INF|                                                                                |mixed_chars="\\\"|\n\\\n\\n\0\\0\"|"|

Stderr:
`))
}

func TestFloats(t *testing.T) {
	lgr := itlog.New(StdoutBuffer, itlog.LevelInfo)
	lgr.Clone().WithFloat32("float32.whole", 1).Info().Msg("")
	lgr.Clone().WithFloat64("float64.whole", 1).Info().Msg("")
	lgr.Clone().WithFloat32("float32.arbitrary", 23.1245).Info().Msg("")
	lgr.Clone().WithFloat64("float64.arbitrary", 23.1245).Info().Msg("")
	lgr.Clone().WithFloat32("float32.NaN", float32(math.NaN())).Info().Msg("")
	lgr.Clone().WithFloat64("float64.NaN", math.NaN()).Info().Msg("")
	lgr.Clone().WithFloat32("float32.PositiveInf", float32(math.Inf(1))).Info().Msg("")
	lgr.Clone().WithFloat64("float64.PositiveInf", math.Inf(1)).Info().Msg("")
	lgr.Clone().WithFloat32("float32.NegativeInf", float32(math.Inf(-1))).Info().Msg("")
	lgr.Clone().WithFloat64("float64.NegativeInf", math.Inf(-1)).Info().Msg("")
	lgr.Clone().WithFloat32("float32.Max", math.MaxFloat32).Info().Msg("")
	lgr.Clone().WithFloat32("float32.Min", -math.MaxFloat32).Info().Msg("")
	lgr.Clone().WithFloat64("float64.Max", math.MaxFloat64).Info().Msg("")
	lgr.Clone().WithFloat64("float64.Min", -math.MaxFloat64).Info().Msg("")
	lgr.Clone().WithFloat32("float32.SmallestNonzero", math.SmallestNonzeroFloat32).Info().Msg("")
	lgr.Clone().WithFloat64("float64.SmallestNonzero", math.SmallestNonzeroFloat64).Info().Msg("")
	lgr.Info().Msg("------------------------------------------------------------------------------------")
	lgr.Info().Float32("float32.whole", 1).Msg("")
	lgr.Info().Float64("float64.whole", 1).Msg("")
	lgr.Info().Float32("float32.arbitrary", 1.1245).Msg("")
	lgr.Info().Float64("float64.arbitrary", 1.1245).Msg("")
	lgr.Info().Float32("float32.NaN", float32(math.NaN())).Msg("")
	lgr.Info().Float64("float64.NaN", math.NaN()).Msg("")
	lgr.Info().Float32("float32.PositiveInf", float32(math.Inf(1))).Msg("")
	lgr.Info().Float64("float64.PositiveInf", math.Inf(1)).Msg("")
	lgr.Info().Float32("float32.NegativeInf", float32(math.Inf(-1))).Msg("")
	lgr.Info().Float64("float64.NegativeInf", math.Inf(-1)).Msg("")
	lgr.Info().Float32("float32.Max", math.MaxFloat32).Msg("")
	lgr.Info().Float32("float32.Min", -math.MaxFloat32).Msg("")
	lgr.Info().Float64("float64.Max", math.MaxFloat64).Msg("")
	lgr.Info().Float64("float64.Min", -math.MaxFloat64).Msg("")
	lgr.Info().Float32("float32.SmallestNonzero", math.SmallestNonzeroFloat32).Msg("")
	lgr.Info().Float64("float64.SmallestNonzero", math.SmallestNonzeroFloat64).Msg("")

	check(t, snap.Init(`Stdout:
2000-01-31T23:59:59Z|INF|                                                                                |float32.whole=1e+00|
2000-01-31T23:59:59Z|INF|                                                                                |float64.whole=1e+00|
2000-01-31T23:59:59Z|INF|                                                                                |float32.arbitrary=2.31245e+01|
2000-01-31T23:59:59Z|INF|                                                                                |float64.arbitrary=2.31245e+01|
2000-01-31T23:59:59Z|INF|                                                                                |float32.NaN=NaN|
2000-01-31T23:59:59Z|INF|                                                                                |float64.NaN=NaN|
2000-01-31T23:59:59Z|INF|                                                                                |float32.PositiveInf=+Inf|
2000-01-31T23:59:59Z|INF|                                                                                |float64.PositiveInf=+Inf|
2000-01-31T23:59:59Z|INF|                                                                                |float32.NegativeInf=-Inf|
2000-01-31T23:59:59Z|INF|                                                                                |float64.NegativeInf=-Inf|
2000-01-31T23:59:59Z|INF|                                                                                |float32.Max=3.4028235e+38|
2000-01-31T23:59:59Z|INF|                                                                                |float32.Min=-3.4028235e+38|
2000-01-31T23:59:59Z|INF|                                                                                |float64.Max=1.7976931348623157e+308|
2000-01-31T23:59:59Z|INF|                                                                                |float64.Min=-1.7976931348623157e+308|
2000-01-31T23:59:59Z|INF|                                                                                |float32.SmallestNonzero=1e-45|
2000-01-31T23:59:59Z|INF|                                                                                |float64.SmallestNonzero=5e-324|
2000-01-31T23:59:59Z|INF|--------------------------------------------------------------------------------|
2000-01-31T23:59:59Z|INF|                                                                                |float32.whole=1e+00|
2000-01-31T23:59:59Z|INF|                                                                                |float64.whole=1e+00|
2000-01-31T23:59:59Z|INF|                                                                                |float32.arbitrary=1.1245e+00|
2000-01-31T23:59:59Z|INF|                                                                                |float64.arbitrary=1.1245e+00|
2000-01-31T23:59:59Z|INF|                                                                                |float32.NaN=NaN|
2000-01-31T23:59:59Z|INF|                                                                                |float64.NaN=NaN|
2000-01-31T23:59:59Z|INF|                                                                                |float32.PositiveInf=+Inf|
2000-01-31T23:59:59Z|INF|                                                                                |float64.PositiveInf=+Inf|
2000-01-31T23:59:59Z|INF|                                                                                |float32.NegativeInf=-Inf|
2000-01-31T23:59:59Z|INF|                                                                                |float64.NegativeInf=-Inf|
2000-01-31T23:59:59Z|INF|                                                                                |float32.Max=3.4028235e+38|
2000-01-31T23:59:59Z|INF|                                                                                |float32.Min=-3.4028235e+38|
2000-01-31T23:59:59Z|INF|                                                                                |float64.Max=1.7976931348623157e+308|
2000-01-31T23:59:59Z|INF|                                                                                |float64.Min=-1.7976931348623157e+308|
2000-01-31T23:59:59Z|INF|                                                                                |float32.SmallestNonzero=1e-45|
2000-01-31T23:59:59Z|INF|                                                                                |float64.SmallestNonzero=5e-324|

Stderr:
`))
}

func TestArray(t *testing.T) {
	lgr := itlog.New(StdoutBuffer, itlog.LevelInfo)
	lgr.Info().Strs("rick.astley", "never", "gonna", "give", "you", "up").Msg("")
	check(t, snap.Init(`Stdout:
2000-01-31T23:59:59Z|INF|                                                                                |rick.astley=[ "never" "gonna" "give" "you" "up" ]|

Stderr:
`))
}

func TestNilLogger(t *testing.T) {
	lgr := itlog.New(StdoutBuffer, itlog.LevelDisabled).
		WithStr("", "").
		WithErr("", nil).
		WithBool("", true).
		WithInt("", int(^uint(0)>>1)).
		WithInt8("", 1<<7-1).
		WithInt16("", 1<<15-1).
		WithInt32("", 1<<31-1).
		WithInt64("", 1<<63-1).
		WithUint8("", 1<<8-1).
		WithUint16("", 1<<16-1).
		WithUint32("", 1<<32-1).
		WithUint64("", 1<<64-1).
		WithUint("", ^uint(0)).
		WithFloat32("", 0).
		WithFloat64("", 0).
		WithTime("", time.Time{}).
		WithData(nil, nil)

	lgr.Debug().Msg("")
	lgr.Info().Msg("")
	lgr.Warn().Msg("")
	lgr.Error().
		Str("", "").
		Strs("").
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
		Float32("", 0).
		Float64("", 0).
		Bool("", true).
		Err(errors.New("")).
		Errs(errors.New("")).
		Time("", time.Time{}).
		Data(nil, nil).
		Msg("")
	lgr.Info().Begin("")
	lgr.Info().Done("")
	check(t, snap.Init(`Stdout:

Stderr:
`))
}

func TestLevelThresholds(t *testing.T) {
	lgr := itlog.New(StdoutBuffer, itlog.LevelDisabled-1).
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
	check(t, snap.Init(`Stdout:

Stderr:
`))
}

func TestEmpty(t *testing.T) {
	lgr := itlog.New(StdoutBuffer, itlog.LevelError)
	lgr = lgr.Clone()
	lgr = lgr.
		WithStr("", "").
		WithErr("", nil).
		WithBool("", true).
		WithInt8("", 1<<7-1).
		WithInt16("", 1<<15-1).
		WithInt32("", 1<<31-1).
		WithInt64("", 1<<63-1).
		Clone().
		WithInt("", int(^uint(0)>>1)).
		WithUint8("", 1<<8-1).
		WithUint16("", 1<<16-1).
		WithUint32("", 1<<32-1).
		WithUint64("", 1<<64-1).
		WithUint("", ^uint(0)).
		WithFloat32("", float32(math.NaN())).
		WithFloat32("", float32(math.Inf(1))).
		WithFloat32("", float32(math.Inf(-1))).
		WithFloat32("", math.MaxFloat32).
		WithFloat32("", -math.MaxFloat32).
		WithFloat32("", math.SmallestNonzeroFloat32).
		WithFloat64("", math.NaN()).
		WithFloat64("", math.Inf(1)).
		WithFloat64("", math.Inf(-1)).
		WithFloat64("", math.MaxFloat64).
		WithFloat64("", -math.MaxFloat64).
		WithFloat64("", math.SmallestNonzeroFloat64).
		Clone()

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
	check(t, snap.Init(`Stdout:
2000-01-31T23:59:59Z|ERR|                                                                                |__EMPTY__="__EMPTY__"|__EMPTY__=true|__EMPTY__=127|__EMPTY__=32767|__EMPTY__=2147483647|__EMPTY__=9223372036854775807|__EMPTY__=9223372036854775807|__EMPTY__=255|__EMPTY__=65535|__EMPTY__=4294967295|__EMPTY__=18446744073709551615|__EMPTY__=-1|__EMPTY__=NaN|__EMPTY__=+Inf|__EMPTY__=-Inf|__EMPTY__=3.4028235e+38|__EMPTY__=-3.4028235e+38|__EMPTY__=1e-45|__EMPTY__=NaN|__EMPTY__=+Inf|__EMPTY__=-Inf|__EMPTY__=1.7976931348623157e+308|__EMPTY__=-1.7976931348623157e+308|__EMPTY__=5e-324|__EMPTY__="__EMPTY__"|__EMPTY__=18446744073709551615|__EMPTY__=-9223372036854775808|__EMPTY__=-128|__EMPTY__=-32768|__EMPTY__=-2147483648|__EMPTY__=-9223372036854775808|__EMPTY__=18446744073709551615|__EMPTY__=255|__EMPTY__=65535|__EMPTY__=4294967295|__EMPTY__=true|

Stderr:
`))
}

func TestNilWriter(t *testing.T) {
	lgr := itlog.New(nil, itlog.LevelInfo)
	if lgr != nil {
		t.Fatal("Initialized logger is not nil when writer is nil")
	}
}

func TestErrorConvenience(t *testing.T) {
	lgr := itlog.New(StdoutBuffer, itlog.LevelError)
	lgr.Error().Msg("")
	lgr.Error(nil).Msg("")
	lgr.Error(nil, nil, nil).Msg("")
	lgr.Error(errors.New("foo")).Msg("")
	lgr.Error(nil, errors.New("bar"), nil).Msg("")
	lgr.Error(errors.New("foo"), errors.New("bar"), errors.New("baz")).Msg("")

	check(t, snap.Init(`Stdout:
2000-01-31T23:59:59Z|ERR|                                                                                |
2000-01-31T23:59:59Z|ERR|                                                                                |
2000-01-31T23:59:59Z|ERR|                                                                                |
2000-01-31T23:59:59Z|ERR|                                                                                |error="foo"|
2000-01-31T23:59:59Z|ERR|                                                                                |error="bar"|
2000-01-31T23:59:59Z|ERR|                                                                                |error="foo"|error="bar"|error="baz"|

Stderr:
`))
}

func TestBeginDone(t *testing.T) {
	lgr := itlog.New(StdoutBuffer, itlog.LevelInfo)
	lgr.Info().Begin("validating cache")
	lgr.Info().Done("validating cache")
	check(t, snap.Init(`Stdout:
2000-01-31T23:59:59Z|INF|begin validating cache                                                          |
2000-01-31T23:59:59Z|INF|done  validating cache                                                          |

Stderr:
`))
}
