# itlog

## Benchmarks

~5 million logs per second! There's alot of low hanging fruit for optimization still.

```
$ go test ./itlog -bench=. -benchmem # assertions are enabled
goos: darwin
goarch: arm64
pkg: golang-snacks/itlog
cpu: Apple M4
BenchmarkLoggerDebugSimple-10                    6040744               178.2 ns/op           388 B/op          0 allocs/op
BenchmarkLoggerInfoSimple-10                     7982898               174.9 ns/op           336 B/op          0 allocs/op
BenchmarkLoggerWarnSimple-10                     7987365               292.3 ns/op           672 B/op          0 allocs/op
BenchmarkLoggerErrorSimple-10                    5851641               206.8 ns/op             0 B/op          0 allocs/op
BenchmarkLoggerWithStrContext-10                 5358642               224.3 ns/op             0 B/op          0 allocs/op
BenchmarkLoggerWithIntAndBoolContext-10          1000000              2717 ns/op           10737 B/op          0 allocs/op
BenchmarkLoggerWithErrs-10                       5023482               245.0 ns/op             0 B/op          0 allocs/op
BenchmarkLoggerLongMessage-10                    6606853               187.2 ns/op             0 B/op          0 allocs/op
BenchmarkLoggerMultipleDataCalls-10              5481381               215.2 ns/op             0 B/op          0 allocs/op
BenchmarkLoggerListContext-10                    4940658               253.3 ns/op             0 B/op          0 allocs/op
BenchmarkLoggerInheritance-10                    5628788               208.7 ns/op             0 B/op          0 allocs/op
BenchmarkLoggerEscaping-10                       5699820              1259 ns/op            3767 B/op          0 allocs/op
BenchmarkLoggerUint64AndInt64-10                 4981610               225.4 ns/op             0 B/op          0 allocs/op
BenchmarkLogger10MixedFields-10                  4259754               280.3 ns/op             0 B/op          0 allocs/op
PASS
ok      golang-snacks/itlog     29.381s
```
