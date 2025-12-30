//go:build !disable_assertions && !disable_x_assertions

package invariant

import (
	"errors"
	"fmt"
)

/*
XAlways evaluates fn and calls assertionFailureCallback if it returns false. It
is designed for use cases where you want to perform expensive validations that
can be disabled in production builds using the `disable_assertions`
build tag.

	expensiveFn := func() bool { ... }
	// expensiveFn is still evaluated but boolean check is a noop under disable_assertions
	invariant.Always(expensiveFn())


	// expensiveFn itself will be a noop under disable_assertions
	invariant.XAlways(expensiveFn)

Be wary of this if you rely on side effects produced by fn. Rule of thumb would
be to ensure that fn is pure or idempotent.
*/
//go:noinline
func XAlways(fn func() bool, msg string) {
	if fn() {
		registerAssertion()
	} else {
		assertionFailureCallback(fmt.Sprintf("%s: %s\n", AssertionFailureMsgPrefix, msg))
	}
}

//go:noinline
func XSometimes(fn func() bool, msg string) {
	if !IsRunningUnderGoTest || !fn() {
		return
	}
	registerAssertion()
}

// XAlwaysNil evaluates fn and calls assertionFailureCallback if the result is not nil.
//
//go:noinline
func XAlwaysNil(fn func() interface{}, msg string) {
	x := fn()
	if x == nil {
		registerAssertion()
	} else {
		assertionFailureCallback(fmt.Sprintf("%s: expected nil. got %v. %s\n", AssertionFailureMsgPrefix, x, msg))
	}
}

// XAlwaysErrIs evaluates fn and calls assertionFailureCallback if the returned error is not in targets.
//
//go:noinline
func XAlwaysErrIs(fn func() error, targets []error, msg string) {
	Always(len(targets) > 0, "invariant.XAlwaysErrIs requires at least one target")
	for _, t := range targets {
		Always(t != nil, "All invariant.XAlwaysErrIs targets must not be nil")
	}
	actual := fn()
	for _, t := range targets {
		if errors.Is(actual, t) {
			registerAssertion()
			return
		}
	}
	assertionFailureCallback(fmt.Sprintf("%s: error did not match any targets. got %q. %s\n", AssertionFailureMsgPrefix, actual, msg))
}

// XAlwaysErrIsNot evaluates fn and calls assertionFailureCallback if the returned error matches any target.
//
//go:noinline
func XAlwaysErrIsNot(fn func() error, targets []error, msg string) {
	Always(len(targets) > 0, "invariant.XAlwaysErrIsNot requires at least one target")
	for _, t := range targets {
		Always(t != nil, "All invariant.XAlwaysErrIsNot targets must not be nil")
	}
	actual := fn()
	for _, t := range targets {
		if errors.Is(actual, t) {
			assertionFailureCallback(fmt.Sprintf("error unexpectedly matched a target. got %q. %s\n", actual, msg))
			return
		}
	}
	registerAssertion()
}
