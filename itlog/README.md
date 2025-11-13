# itlog

A zero-allocation structured logger for Go, inspired by Zerolog but 30× smaller and optimized for human-readable output.
Its accomplishments are:

1. Smaller footprint (500 SLOC vs Zerolog's 16k SLOC)
2. Similar core API as Zerolog
3. Easily vendorable (MIT licensed)
4. Zero heap allocations
5. Performant for human-readable format (80-250 nanoseconds per log)

```go
package main

import "golang-snacks/itlog"

func main() {
	logger := itlog.New(itlog.LevelInfo)
	logger.WithStr("name", "James").WithStr("email", "iwillhackyou@proton.mail")
	{
		logger := logger.WithInt("FAANG companies hacked", 369)
		logger.Info().Msg("This is a deep copy of the parent logger.")
	}
	logger.Info().Msg("This logger's buffer was not mutated")
}

// $ go run .
// 2025-11-05T23:10:33Z|INF|This is a deep copy of the parent logger.                                       |FAANG companies hacked=369|
// 2025-11-05T23:10:33Z|INF|This logger's buffer was not mutated                                            |
```

## Design


### Format

```
.......Header.....|.Body..\n
time|level|message|context\n
```

- The format is split into the Header and Body.
- The header is a fixed-width chunk, holding fields that are present in all
  logs. It is split into sub-components (1) time, (2) log level, (3) and
  message, all separated by the ComponentSeparator.
- The body is a variable-width chunk, holding dynamically appended fields. It
  consists of key=value pairs separated by ComponentSeparator and delimited by
  ContextKeyValueSeparator. Keys with multiple values are appended as individual
  pairs len(values) times in the context, sharing the same key.

There are only four escaped characters, ComponentSeparator,
ContextKeyValueSeparator, null bytes, and newlines:

- Since raw newlines (0x0A) are turned into the string `\n`, we must distinguish
  them from literal `\n` sequences already present in the data to make the
  encoding reversible. Moreover, this ensures that the only raw newlines in the
  logs are log record delimiters; this is imperative for preserving visual
  structure. It prevents user data containing newlines from visually splitting a
  single record into multiple lines when displayed in GUIs.
- Every null byte in the data is escaped, ensuring that any unescaped null byte
  appearing in the logs can be safely interpreted as data corruption.

### No colored output

At first, I implemented colored output. In practice however, the colors are not
all that useful when the context gets large and your terminal screen is filled
with text that hard wrap, breaking visual alignment of the logs. It's better to
save the logs in a file and explore them in your editor. Another benefit is that
this simplifies the implementation further.

## Benchmarks

```
$ go test ./itlog -bench=. -count=20 -benchtime=0.01s -benchmem > benchmark.txt
$ go test ./itlog -bench=. -count=20 -benchtime=0.01s -benchmem -tags=disable_assertions > benchmark_disable_assertions.txt
$ benchstat benchmark.txt benchmark_disable_assertions.txt
goos: darwin
goarch: arm64
pkg: golang_snacks/itlog
cpu: Apple M4
                               │ benchmark.txt │   benchmark_disable_assertions.txt   │
                               │    sec/op     │    sec/op     vs base                │
LoggerInfoSimple-10               155.95n ± 2%   87.95n ± 53%  -43.60% (p=0.000 n=20)
LoggerErrorSimple-10               184.8n ± 1%   110.1n ±  1%  -40.41% (p=0.000 n=20)
LoggerWithStrContext-10            233.9n ± 1%   128.3n ±  2%  -45.15% (p=0.000 n=20)
LoggerWithIntAndBoolContext-10     215.5n ± 1%   115.5n ±  1%  -46.43% (p=0.000 n=20)
LoggerWithErrs-10                  227.7n ± 1%   133.6n ±  1%  -41.33% (p=0.000 n=20)
LoggerLongMessage-10              137.85n ± 1%   76.31n ±  1%  -44.65% (p=0.000 n=20)
LoggerMultipleDataCalls-10         228.0n ± 1%   117.4n ±  1%  -48.50% (p=0.000 n=20)
LoggerListContext-10               287.2n ± 1%   159.9n ±  1%  -44.35% (p=0.000 n=20)
LoggerInheritance-10               195.8n ± 1%   105.7n ±  1%  -46.04% (p=0.000 n=20)
LoggerEscaping-10                  207.6n ± 2%   114.5n ±  1%  -44.85% (p=0.000 n=20)
LoggerUint64AndInt64-10            227.8n ± 1%   124.0n ± 20%  -45.58% (p=0.000 n=20)
Logger10MixedFields-10             417.0n ± 1%   234.5n ±  1%  -43.75% (p=0.000 n=20)
geomean                            218.0n        120.8n        -44.59%

                               │ benchmark.txt │  benchmark_disable_assertions.txt   │
                               │     B/op      │    B/op     vs base                 │
LoggerInfoSimple-10               0.000 ± 0%     0.000 ±  ?       ~ (p=0.514 n=20)
LoggerErrorSimple-10              0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=20)
LoggerWithStrContext-10           0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=20)
LoggerWithIntAndBoolContext-10    0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=20) ¹
LoggerWithErrs-10                 0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=20)
LoggerLongMessage-10              0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=20)
LoggerMultipleDataCalls-10        0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=20) ¹
LoggerListContext-10              0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=20) ¹
LoggerInheritance-10              0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=20)
LoggerEscaping-10                 0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=20) ¹
LoggerUint64AndInt64-10           0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=20)
Logger10MixedFields-10            0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=20) ¹
geomean                                      ²               +0.00%                ²
¹ all samples are equal
² summaries must be >0 to compute geomean

                               │ benchmark.txt │  benchmark_disable_assertions.txt   │
                               │   allocs/op   │ allocs/op   vs base                 │
LoggerInfoSimple-10               0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=20) ¹
LoggerErrorSimple-10              0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=20) ¹
LoggerWithStrContext-10           0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=20) ¹
LoggerWithIntAndBoolContext-10    0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=20) ¹
LoggerWithErrs-10                 0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=20) ¹
LoggerLongMessage-10              0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=20) ¹
LoggerMultipleDataCalls-10        0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=20) ¹
LoggerListContext-10              0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=20) ¹
LoggerInheritance-10              0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=20) ¹
LoggerEscaping-10                 0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=20) ¹
LoggerUint64AndInt64-10           0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=20) ¹
Logger10MixedFields-10            0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=20) ¹
geomean                                      ²               +0.00%                ²
¹ all samples are equal
² summaries must be >0 to compute geomean
```
