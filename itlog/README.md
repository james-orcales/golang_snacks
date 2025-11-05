# itlog

## Benchmarks

~6 million logs per second! There's alot of low hanging fruit for optimization still.

```
$ go test ./itlog -bench=. -benchmem # assertions are disabled
goos: darwin
goarch: arm64
pkg: golang-snacks/itlog
cpu: Apple M4
BenchmarkLoggerDebugSimple-10                    6961448               150.7 ns/op           337 B/op          0 allocs/op
BenchmarkLoggerInfoSimple-10                     9165280               148.8 ns/op           292 B/op          0 allocs/op
BenchmarkLoggerWarnSimple-10                     9515539               180.3 ns/op           564 B/op          0 allocs/op
BenchmarkLoggerErrorSimple-10                    7901550               153.2 ns/op             0 B/op          0 allocs/op
BenchmarkLoggerWithStrContext-10                 7045986               615.0 ns/op          1523 B/op          0 allocs/op
BenchmarkLoggerWithIntAndBoolContext-10          5360566               227.4 ns/op             0 B/op          0 allocs/op
BenchmarkLoggerWithErrs-10                       5061234               233.9 ns/op             0 B/op          0 allocs/op
BenchmarkLoggerLongMessage-10                    8624770               143.7 ns/op             0 B/op          0 allocs/op
BenchmarkLoggerMultipleDataCalls-10              6354640               184.6 ns/op             0 B/op          0 allocs/op
BenchmarkLoggerListContext-10                    1000000              6294 ns/op           21474 B/op          0 allocs/op
BenchmarkLoggerInheritance-10                    6235153               186.4 ns/op             0 B/op          0 allocs/op
BenchmarkLoggerEscaping-10                       6302113               187.3 ns/op             0 B/op          0 allocs/op
BenchmarkLoggerUint64AndInt64-10                 6071142               180.2 ns/op             0 B/op          0 allocs/op
BenchmarkLogger10MixedFields-10                  4630480               260.6 ns/op             0 B/op          0 allocs/op
PASS
ok      golang-snacks/itlog     28.783s
```
