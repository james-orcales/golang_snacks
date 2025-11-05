# itlog

A zero-allocation structured logger for Go, inspired by Zerolog but 30× smaller and optimized for human-readable output.
Its accomplishments are:

1. Smaller footprint (500 SLOC vs Zerolog's 16k SLOC)
2. Similar core API as Zerolog
3. Easily vendorable (MIT licensed)
4. Zero heap allocations
5. Performant for human-readable format (100-300 nanoseconds per log)

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
- The header is a fixed-width chunk, holding fields that are present in all logs. It is split into sub-components (1) time, (2) log level, (3) and
message, all separated by the ComponentSeparator.
- The body is a variable-width chunk, holding dynamically appended fields. It consists of key=value pairs separated by ComponentSeparator and delimited by
ContextKeyValueSeparator. Keys with multiple values are appended as individual pairs len(values) times in the context, sharing the same key.

A key characteristic of this design is that it simplifies escaping. There are only two special characters, ComponentSeparator and ContextKeyValueSeparator.

### No colored output

At first, I implemented colored output. In practice however, the colors are not all that useful when the context gets large and your terminal screen is
filled with text that hard wrap, breaking visual alignment of the logs. It's better to save the logs in a file and explore them in your editor. Another
benefit is that this simplifies the implementation further.

## Benchmarks

```
$ go test ./itlog -bench=. -count=30 -benchtime=0.05s -benchmem > benchmark_base.txt
$ benchstat benchmark_base.txt benchmark_base_no_assertions.txt
goos: darwin
goarch: arm64
pkg: golang-snacks/itlog
cpu: Apple M4
                               │ benchmark_base.txt │ benchmark_base_no_assertions.txt   │
                               │     nanoseconds/op │ nanoseconds/op vs base             │
LoggerInfoSimple-10                    135.1  ±  0%   112.7  ±  1%  -16.59% (p=0.000 n=30)
LoggerErrorSimple-10                   148.0  ±  1%   130.0  ±  1%  -12.16% (p=0.000 n=30)
LoggerWithStrContext-10                168.4  ± 11%   179.0  ± 16%        ~ (p=0.379 n=30)
LoggerWithIntAndBoolContext-10         173.6  ±  1%   183.5  ±  7%        ~ (p=0.154 n=30)
LoggerWithErrs-10                      192.2  ±  5%   184.2  ±  8%   -4.14% (p=0.009 n=30)
LoggerLongMessage-10                   136.9  ±  1%   117.5  ±  2%  -14.17% (p=0.000 n=30)
LoggerMultipleDataCalls-10             167.5  ±  2%   148.5  ±  1%  -11.37% (p=0.000 n=30)
LoggerListContext-10                   201.9  ±  1%   185.6  ±  1%   -8.10% (p=0.000 n=30)
LoggerInheritance-10                   157.0  ±  1%   143.4  ±  1%   -8.69% (p=0.000 n=30)
LoggerEscaping-10                      175.5  ± 12%   194.2  ±  6%  +10.62% (p=0.026 n=30)
LoggerUint64AndInt64-10                232.7  ±  2%   172.6  ±  3%  -25.81% (p=0.000 n=30)
Logger10MixedFields-10                 261.0  ±  1%   244.4  ±  0%   -6.36% (p=0.000 n=30)
geomean                                175.8          162.4          -7.60%

                               │ benchmark_base.txt │  benchmark_base_no_assertions.txt   │
                               │        B/op        │    B/op     vs base                 │
LoggerInfoSimple-10                    0.000 ± 0%     0.000 ± 0%       ~ (p=0.982 n=30)
LoggerErrorSimple-10                   0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=30)
LoggerWithStrContext-10                0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=30)
LoggerWithIntAndBoolContext-10         0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=30) ¹
LoggerWithErrs-10                      0.000 ± 0%     0.000 ± 0%       ~ (p=0.334 n=30)
LoggerLongMessage-10                   0.000 ± 0%     0.000 ± 0%       ~ (p=0.334 n=30)
LoggerMultipleDataCalls-10             0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=30) ¹
LoggerListContext-10                   0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=30) ¹
LoggerInheritance-10                   0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=30) ¹
LoggerEscaping-10                      0.000 ± 0%     0.000 ± 0%       ~ (p=0.334 n=30)
LoggerUint64AndInt64-10                0.000 ± 0%     0.000 ± 0%       ~ (p=0.334 n=30)
Logger10MixedFields-10                 0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=30) ¹
geomean                                           ²               +0.00%                ²
¹ all samples are equal
² summaries must be >0 to compute geomean

                               │ benchmark_base.txt │  benchmark_base_no_assertions.txt   │
                               │     allocs/op      │ allocs/op   vs base                 │
LoggerInfoSimple-10                    0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=30) ¹
LoggerErrorSimple-10                   0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=30) ¹
LoggerWithStrContext-10                0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=30) ¹
LoggerWithIntAndBoolContext-10         0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=30) ¹
LoggerWithErrs-10                      0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=30) ¹
LoggerLongMessage-10                   0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=30) ¹
LoggerMultipleDataCalls-10             0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=30) ¹
LoggerListContext-10                   0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=30) ¹
LoggerInheritance-10                   0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=30) ¹
LoggerEscaping-10                      0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=30) ¹
LoggerUint64AndInt64-10                0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=30) ¹
Logger10MixedFields-10                 0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=30) ¹
geomean                                           ²               +0.00%                ²
¹ all samples are equal
² summaries must be >0 to compute geomean
```
