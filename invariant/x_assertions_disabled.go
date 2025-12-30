//go:build disable_assertions || disable_x_assertions

package invariant

func XAlways(fn func() bool, msg string) {
}

func XSometimes(fn func() bool, msg string) {
}

func XAlwaysNil(fn func() interface{}, msg string) {
}

func XAlwaysErrIs(fn func() error, targets []error, msg string) {
}

func XAlwaysErrIsNot(fn func() error, targets []error, msg string) {
}
