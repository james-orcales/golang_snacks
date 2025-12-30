//go:build disable_assertions

package invariant

import (
	"iter"
)

func registerAssertion() {
}

func RegisterPackagesForAnalysis(dirs ...string) {
}

func AnalyzeAssertionFrequency() {
}

func Until(_ int) iter.Seq[int] {
	return func(yield func(int) bool) {
		iteration := 0
		for {
			if !yield(iteration) {
				return
			}
			iteration++
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
