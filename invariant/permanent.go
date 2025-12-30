package invariant

import (
	"fmt"
	"iter"
	"math/rand/v2"
	"os"
	"path"
	"runtime"
	"strings"
	"github.com/james-orcales/golang_snacks/xdebug"
)

const (
	// Used to detect panics caused by assertion failures
	//
	//	defer func() {
	//		if err := recover(); err != nil {
	//			if strErr, ok := err.(string); ok && strings.HasPrefix(strErr, invariant.AssertionFailureMsgPrefix) {
	//				// handle assertion failure
	//			}
	//		}
	//	}()
	AssertionFailureMsgPrefix = "🚨 Assertion Failure 🚨"
)

var (
	// msg is already prefixed with AssertionFailurePrefix here. If the user msg is empty then
	// it also contains emptyMessageIndicator.
	AssertionFailureHook    = func(msg string) {}
	AssertionFailureIsFatal = false
	AssertionFailureIsFatal = true
)

// WARN: Callers rely on this callback to implicitly terminate control flow on failure (via
// panic or os.Exit).
func assertionFailureCallback(msg string) {
	AssertionFailureHook(msg)
	if AssertionFailureIsFatal {
		xdebug.FprintStackTrace(os.Stderr, 1)
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	} else {
		panic(msg)
	}
}

var IsRunningUnderGoTest = func() bool {
	v := false
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.") {
			v = true
			break
		}
	}
	return v
}()

var IsRunningUnderGoFuzz = func() bool {
	v := false
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.fuzz") {
			v = true
			break
		}
	}
	return v
}()

var IsRunningUnderGoBenchmark = func() bool {
	v := false
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.bench") {
			v = true
			break
		}
	}
	return v
}()

//go:noinline
func Ensure(cond bool, msg string) {
	if cond {
		registerAssertion()
	} else {
		assertionFailureCallback(msg)
	}
}

// GameLoop provides an explicit infinite loop sequence, similar to `for {}`. This makes infinite
// loops explicit, which is useful if `for {}` is banned in CI/CD or code reviews.
//
// Usage:
//
//	for range invariant.GameLoop() {
//		rl.Render()
//	}
//
// Each iteration yields an increasing integer starting from 0.
func GameLoop() iter.Seq[int] {
	return func(yield func(int) bool) {
		for iteration := 0; true; iteration++ {
			if !yield(iteration) {
				return
			}
		}
	}
}

// InjectFault randomly triggers a fault for testing purposes, based on the given percentage.
// Only active when running under Go fuzzing (`IsRunningUnderGoFuzz`).
//
// Example usage:
//
//	err := fallible()
//	if err != nil || invariant.InjectFault(30) {
//	    handleError()
//	}
//
// The `percent` argument specifies the probability (0–100) that a fault is injected.
func InjectFault(percent int) bool {
	return IsRunningUnderGoFuzz && rand.IntN(100) < percent
}
