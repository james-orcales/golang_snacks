# itlog

A zero-allocation structured logger for Go, inspired by Zerolog but 20× smaller and optimized for human-readable output.
Its accomplishments are:

1. Smaller footprint (800 SLOC vs Zerolog's 16k SLOC)
2. Similar core API as Zerolog
3. Easily vendorable (MIT licensed)
4. Zero heap allocations
5. Performant for human-readable format (25 million logs per second)

```go
package main

import "golang-snacks/itlog"

func main() {
	logger := itlog.New(itlog.LevelInfo)
	logger.WithStr("name", "James").WithStr("email", "iwillhackyou@proton.mail")
	{
		logger := logger.WithInt("FAANG_companies_hacked", 369)
		logger.Info().Msg("This is a deep copy of the parent logger.")
	}
	logger.Info().Msg("This logger's buffer was not mutated")
}

// $ go run .
// 2025-11-05T23:10:33Z|INF|This is a deep copy of the parent logger.                                       |FAANG_companies_hacked=369|
// 2025-11-05T23:10:33Z|INF|This logger's buffer was not mutated                                            |
```

## Benchmarks

These are the EXACT same benchmarks used by Zerolog. Note some benchmarks were
excluded since they're unsupported yet.

```
$ go test ./itlog   -bench=. -count=20 -benchtime=0.01s -benchmem -tags=disable_assertions > itlog_bench
$ go test ./zerolog -bench=. -count=20 -benchtime=0.01s -benchmem > zerolog_bench
$ benchstat itlog_bench zerolog_bench
goos: darwin
goarch: arm64
pkg: golang_snacks/itlog
cpu: Apple M4
                          │ itlog_bench        |      zerolog_bench             │
                          │ nanosec/op         │ nanosec/op     vs base         │
LogEmpty-10                 23.035n ± 22%        6.853n ±  6%  -70.25% (p=0.000 n=20)
Disabled-10                 0.2980n ±  2%       0.3010n ±  1%        ~ (p=0.180 n=20)
Info-10                      23.64n ± 16%        12.93n ±  3%  -45.34% (p=0.000 n=20)
ContextFields-10             18.36n ±  3%        12.77n ±  5%  -30.40% (p=0.000 n=20)
ContextAppend-10            40.625n ±  2%        3.271n ±  1%  -91.95% (p=0.000 n=20)
LogFields-10                 42.07n ±  7%        24.71n ±  7%  -41.27% (p=0.000 n=20)


                          │ itlog_bench_append │              zerolog_bench              │
                          │     allocs/op      │ allocs/op   vs base                     │
LogEmpty-10                       0.000 ± 0%     0.000 ± 0%         ~ (p=1.000 n=20) ¹
Disabled-10                       0.000 ± 0%     0.000 ± 0%         ~ (p=1.000 n=20) ¹
Info-10                           0.000 ± 0%     0.000 ± 0%         ~ (p=1.000 n=20) ¹
ContextFields-10                  0.000 ± 0%     0.000 ± 0%         ~ (p=1.000 n=20) ¹
ContextAppend-10                  2.000 ± 0%     0.000 ± 0%  -100.00% (p=0.000 n=20)
LogFields-10                      0.000 ± 0%     0.000 ± 0%         ~ (p=1.000 n=20) ¹
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

- Context keys only permit the following characters `[a-zA-Z._]`
- Context string values are surrounded by double quotes `my_foo="your baz"`

### Encoding

Itlog uses a lossy encoding.

**Message:**  

Raw null bytes (`'\x00'`) and newlines (`'\n'`) are replaced with whitespace `' '`.

**Context:**  
Raw null bytes (`'\x00'`) and newlines (`'\n'`) are encoded as the string
literals `\0` and `\n`. 

### No colored output

At first, I implemented colored output. In practice however, the colors are not
all that useful when the context gets large and your terminal screen is filled
with text that hard wrap, breaking visual alignment of the logs. It's better to
save the logs in a file and explore them in your editor. Another benefit is that
this simplifies the implementation further.


