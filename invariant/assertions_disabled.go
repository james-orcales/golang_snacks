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

type _Number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

func Until[T _Number](_ T) iter.Seq[T] {
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
