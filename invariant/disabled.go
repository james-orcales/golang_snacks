//go:build disable_assertions

package invariant

import (
	"iter"
	"testing"
)

func RunTestMain(m *testing.M, dirs ...string) {
}

func RegisterPackagesForAnalysis(dirs ...string) {
}

func AnalyzeAssertionFrequency() {
}

func Until(_ int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for {
			if !yield(iteration) {
				return
			}
		}
	}
}

func Unreachable(msg string) {
}

func Unimplemented(msg string) {
}

func Sometimes(ok bool, msg string) {
}

func Always(cond bool, msg string) {
}

func AlwaysNil(x any, msg string) {
}

func AlwaysErrIs(actual error, targets []error, msg string) {
}

func AlwaysErrIsNot(actual error, targets []error, msg string) {
}

func XSometimes(ok func() bool, msg string) {
}

func XAlways(fn func() bool, msg string) {
}

func XAlwaysNil(fn func() any, msg string) {
}

func XAlwaysErrIs(fn func() error, targets []error, msg string) {
}

func XAlwaysErrIsNot(fn func() error, targets []error, msg string) {
}
