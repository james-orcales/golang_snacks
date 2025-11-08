//go:build disable_assertions

package invariant

var AssertionFailureCallback = func(msg string) {
}

func RegisterPackagesForAnalysis(dirs ...string) {
}

func AnalyzeAssertionFrequency() {
}

func Sometimes(ok bool, msg string) {
}

func Always(cond bool, msg string) {
}

func AlwaysNil(x any, msg string) {
}

func AlwaysErrIs(actual error, targets ...error) {
}

func AlwaysErrIsNot(actual error, targets ...error) {
}

func XSometimes(ok func() bool, msg string) {
}

func XAlways(fn func() bool) {
}

func XAlwaysNil(fn func() any) {
}

func XAlwaysErrIs(fn func() error, targets ...error) {
}

func XAlwaysErrIsNot(fn func() error, targets ...error) {
}
