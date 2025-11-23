package itlog_test

import (
	"errors"
	"io"
	"testing"

	"github.com/james-orcales/golang_snacks/invariant"
	"github.com/james-orcales/golang_snacks/itlog"
)

func FuzzMessage(f *testing.F) {
	seeds := []string{
		"Filled out message buffer with human-readable ascii characters gibberish gibberish",
		"This one is much much much much muchmuch much much much muchmuch much much much much much much much much much longer than the buffer",
		"\x00This =|one \\nhas many \\|escaped \n\\n\nand =|unescaped \x00\x00 \\=reserved characters\n\\n",
		`未熟 無ジョウ されど 美しくあれ\nNo destiny ふさわしく無い\nこんなんじゃきっと物足りない`,
	}
	invariant.Always(func() bool {
		for _, seed := range seeds {
			if len(seed) < itlog.MessageCapacity {
				return false
			}
			f.Add(seed)
		}
		return true
	}(), "All seeds fill the message buffer")

	f.Fuzz(func(t *testing.T, data string) {
		if data == "" {
			return
		}
		logger := itlog.New(io.Discard, itlog.LevelDebug)
		logger.Info().Msg(data)
	})
}

func FuzzEverything(f *testing.F) {
	f.Add(
		"._..this.is_.a...valid___key_..", "\x00未熟 \\=\n無\n\x00ジョウ |\\|されど =美しくあれ\x00\n", true,
		uint64(1<<64-1), int8(1<<7-1), int(1<<63-1), int16(1<<15-1), int32(1<<31-1), int64(1<<63-1),
		uint(1<<64-1), uint8(1<<8-1), uint16(1<<16-1), uint32(1<<32-1),
	)

	f.Add(
		"未熟 無ジョウ されど 美しくあれ", "未熟 無ジョウ されど 美しくあれ", false,
		uint64(0), int8(-1<<7), int(-1<<63), int16(-1<<15), int32(-1<<31), int64(-1<<63),
		uint(0), uint8(0), uint16(0), uint32(0),
	)

	f.Fuzz(func(
		t *testing.T,
		key, val string,
		cond bool,
		u64 uint64,
		i8 int8,
		i int,
		i16 int16,
		i32 int32,
		i64 int64,
		u uint,
		u8 uint8,
		u16 uint16,
		u32 uint32,
	) {
		if itlog.ValidateKey([]byte(key)) != nil {
			return
		}

		lgr := itlog.New(io.Discard, itlog.LevelDebug)

		lgr = lgr.WithStr(key, val).
			WithBool(key, cond).
			WithErr(key, errors.New(val)).
			WithBool(key, cond).
			WithUint64(key, u64).
			WithInt8(key, i8)

		lgr.Debug().
			Str(key, val).
			Uint64(key, u64).
			Int(key, i).
			Int8(key, i8).
			Int16(key, i16).
			Int32(key, i32).
			Int64(key, i64).
			Uint(key, u).
			Uint8(key, u8).
			Uint16(key, u16).
			Uint32(key, u32).
			Bool(key, cond).
			Bool(key, cond).
			Err(errors.New(val)).
			Msg(key)
	})
}
