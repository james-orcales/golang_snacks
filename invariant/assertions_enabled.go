//go:build !disable_assertions

/*
Package invariant provides a property testing framework for Go, designed to
encode and enforce assumptions (invariants) directly within your code.

# Assertion Types

   - **Always** — The property must hold true for *all* inputs or executions.
     To disprove an Always assertion, you only need *one counterexample* where
     the condition is false.

   - **Sometimes** — The property is expected to be *occasionally true* across
     runs. To prove a Sometimes assertion, you need *one example* where the
     condition is true. If it never evaluates to true, the property is disproven.
     Observing its absence requires a program context where all expected states
     to be consistently exercised—which is only feasible in testing environments.

# Implementation

In production, assertions panic/crash on violation.
In test environments, invariant activates a global tracker that records every
assertion that evaluated to true.

Before tests run, you register the packages to analyze (usually just the current
test’s target package). The parser locates all assertions within said package
then pre-registers them in the tracker. As tests execute, successful assertions
(true evaluations) are recorded in a package-global tracker keyed by file and line
number. It’s crucial that the package qualifier name remains `invariant`, since
the parser relies on this identifier to detect assertions.

Next, the analyzer cross-references parsed assertions with those observed at
runtime. Any assertion that never evaluated to true (frequency = 0) is reported
as a “missed invariant,” causing the test suite to fail. If all assertions were
exercised, the analyzer prints a summary showing the *least exercised* assertions.

Invariant therefore provides actionable, frequency-based insight into how
thoroughly your properties have been exercised, revealing the true scope and
effectiveness of your testing suite. By tracking which invariants fire and
which remain untested, it exposes gaps that ordinary unit tests alone would
never highlight, giving a far more comprehensive view of software reliability
than coverage metrics or ad-hoc tests can offer. Additionally, by making the
assumptions of your software explicit, it improves developer experience and
accelerates onboarding, helping new contributors understand the intended
behavior and constraints of the system.
*/

package invariant

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"iter"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const (
	// This library minimizes heap allocations by preferring fixed-size backing arrays for
	// slices. These values determine the size of those arrays and should accommodate typical
	// codebases without frequent resizing.
	leastExercisedInvariantCount = 10
	// maxAssertionsPerPackage is the maximum number of Sometimes, XSometimes, Always*,
	// XAlways*, and Ensure calls in a single package.
	maxAssertionsPerPackage = 2048
	maxGoFilesPerPackage    = 1024
	maxFilePath             = 260
	maxFileLines            = 5 // In digits (99,999 lines)
	assertionIDLength       = maxFilePath + 1 + maxFileLines
)

var (
	// packagesToAnalyze defaults to the current testing package.
	packagesToAnalyze = []string{"."}
	// assertionTracker globally tracks true assertions inside packagesToAnalyze.
	assertionTracker        = make(map[string]*metadata, maxAssertionsPerPackage*len(packagesToAnalyze))
	assertionFrequencyMutex = sync.Mutex{}
)

type metadata struct {
	Frequency int
	Message   string
	Kind      string
}

// registerAssertion records in the package-global assertion tracker that an
// assertion was evaluated as true at the current source location. It tracks the
// file and line number of the call and increments a counter in
// assertionTracker.
//
// This function is a noop outside of a test environment.
//
// It is concurrency-safe and can be called from multiple goroutines.
//
//go:noinline
func registerAssertion() {
	if IsRunningUnderGoBenchmark || IsRunningUnderGoFuzz {
		return
	}
	callers := [1]uintptr{}
	count := runtime.Callers(3, callers[:])
	frame, _ := runtime.CallersFrames(callers[:count]).Next()

	arr := [assertionIDLength]byte{}
	buf := arr[:0]
	buf = append(buf, frame.File...)
	buf = append(buf, ':')
	buf = strconv.AppendInt(buf, int64(frame.Line), 10)

	id := string(buf)
	assertionFrequencyMutex.Lock()
	a, ok := assertionTracker[id]
	if ok {
		a.Frequency++
	}
	assertionFrequencyMutex.Unlock()
}

// RegisterPackagesForAnalysis ensures that only assertions from the tested
// directories are tracked for frequency analysis. Dirs is relative to the
// directory of the caller.
//
// NOTE: All assertions must have their last parameter be the message parameter
//
// NOTE: During fuzzing or benchmarking, assertion tracking is disabled because
// worker processes handle assertions in separate memory spaces. Any registrations
// done there are not synchronized back to the parent test runner
//
// NOTE: Tracking assertions during fuzzing can help gauge how thoroughly your fuzzer exercises the code.
// However, since this implementation tracks assertions at the package level—and most fuzz tests
// focus on specific functions—this is generally sufficient. Full assertion tracking is mainly
// useful when simulating the entire program; in that case, using a dedicated script, unit test,
// or custom fuzz harness is more appropriate.
//
// PERF: Create an instrumentation tool that hardcodes assertion source location, drastically
// improving registration performance at runtime. This enables assertion tracking in fuzz testing.
func RegisterPackagesForAnalysis(dirs ...string) {
	Always(IsRunningUnderGoTest, "RegisterPackagesForAnalysis is only used in testing environments")
	Always(len(packagesToAnalyze) == 1 && packagesToAnalyze[0] == ".", "packagesToAnalyze was set to the current testing package by default")
	if IsRunningUnderGoBenchmark || IsRunningUnderGoFuzz {
		return
	}
	if len(dirs) > 0 {
		packagesToAnalyze = dirs
	}
	// === Absolute Path Conversion ===
	for i, path := range packagesToAnalyze {
		path, err := filepath.Abs(path)
		if err != nil || path == "" {
			panic(fmt.Sprintf("Failed to convert package path to absolute path: %s\n", err))
		}
		packagesToAnalyze[i] = path
	}

	// ===Collection===
	filesArray := [maxGoFilesPerPackage]string{}
	files := filesArray[:0]
	for _, dir := range packagesToAnalyze {
		before := len(files)
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Ext(path) != ".go" {
				return err
			}
			path, err = filepath.Abs(path)
			if err != nil {
				return err
			}
			if d.IsDir() && path != dir {
				return filepath.SkipDir
			}
			if len(path) > len("_test.go") && strings.HasSuffix(path, "_test.go") {
				return nil
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			panic(fmt.Sprintf("Collecting all files to find missed invariants: %s\n", err))
		}
		after := len(files)
		Always(before < after, "The directory contains go files")
	}
	Always(len(files) > 0, "There's at least one file to parse")

	// ===Parsing===
	semaphore := make(chan struct{}, runtime.NumCPU())
	var wg sync.WaitGroup

	for _, path := range files {
		wg.Add(1)
		semaphore <- struct{}{}
		go func(path string) {
			defer func() { <-semaphore; wg.Done() }()
			fset := token.NewFileSet()
			node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if err != nil {
				return
			}
			ast.Inspect(node, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				ident, ok := sel.X.(*ast.Ident)
				if !ok {
					return true
				}
				if ident.Name != "invariant" {
					return true
				}
				msg := ""
				switch sel.Sel.Name {
				case "Sometimes", "XSometimes", "Ensure", "Always", "AlwaysNil", "AlwaysErrIs", "AlwaysErrIsNot",
					"XAlways", "XAlwaysNil", "XAlwaysErrIs", "XAlwaysErrIsNot":
					pos := fset.Position(call.Lparen)
					key := path + ":" + strconv.Itoa(pos.Line)
					Always(len(call.Args) >= 2, "All assertions have at least two parameters")
					literal, ok := call.Args[len(call.Args)-1].(*ast.BasicLit)
					// TODO: include location
					Always(ok, "The last parameter is the assertion message")
					Always(literal.Kind == token.STRING, "The assertion message is a string literal")
					msg = literal.Value[1 : len(literal.Value)-1] // remove quotes

					assertionTracker[key] = &metadata{
						Kind:    sel.Sel.Name,
						Message: msg,
					}
				}
				return true
			})
		}(path)
	}

	wg.Wait()
}

// AnalyzeAssertionFrequency scans the given directories for Sometimes, Always*,
// XAlways* calls that have never evaluated to true and returns their source
// locations and respective messages. To see the stats, enable verbosity with
// `go test ./foo -v`
//
// Usage:
//
//	func TestMain(m *testing.M) {
//		invariant.RunTestMain(m)
//	}
//
// It is critical that you import this package under the name "invariant" as it
// is hardcoded in the analyzer to look for this identifier.
func AnalyzeAssertionFrequency() {
	Always(IsRunningUnderGoTest, "AnalyzeAssertionFrequency is only used for testing")
	Always(len(packagesToAnalyze) > 0, "At least one package was registered for analysis")
	if IsRunningUnderGoBenchmark || IsRunningUnderGoFuzz {
		return
	}

	longestKindWord := 0
	longestMessageLength := 0
	missed := make([]string, 0, len(assertionTracker))
	for location, metadata := range assertionTracker {
		Always(location != "", "All assertion records have a location")
		if metadata.Frequency == 0 {
			longestKindWord = max(longestKindWord, len(metadata.Kind))
			longestMessageLength = max(longestMessageLength, len(metadata.Message))
			missed = append(missed, location)
		}
	}
	if len(missed) > 0 {
		fmt.Printf("🚨 %d assertions were never true. 🚨\n", len(missed))
		for _, id := range missed {
			fmt.Printf(
				"\t%*s | %-*s | %s\n",
				longestKindWord, assertionTracker[id].Kind,
				longestMessageLength, assertionTracker[id].Message,
				id,
			)
		}
		os.Exit(1)
	}

	// === Analysis ===
	//
	// This is a very simple and “dumb” analysis based solely on absolute
	// assertion frequency. It does not attempt to correlate assertions with
	// specific unit tests or cluster them with each other; any attempt to
	// do so is futile. For that kind of analysis, you’d need
	// instrumentation/static analysis combined with machine learning.
	{
		type scored struct {
			key   string
			count int
		}
		fmt.Printf("Showing up to %d of the least-exercised invariants:\n", leastExercisedInvariantCount)

		longestMessageLength := 0
		longestKindWord := 0

		h := make([]scored, 0, leastExercisedInvariantCount)
		for key, assertion := range assertionTracker {
			Always(key != "", "Assertion location must not be empty")
			longestMessageLength = max(longestMessageLength, len(assertion.Message))
			longestKindWord = max(longestKindWord, len(assertion.Kind))

			if len(h) < leastExercisedInvariantCount {
				h = append(h, scored{key, assertion.Frequency})
				if len(h) == leastExercisedInvariantCount {
					sort.Slice(h, func(i, j int) bool {
						return h[i].count > h[j].count
					})
				}
				continue
			}
			if assertion.Frequency < h[0].count {
				h[0] = scored{key, assertion.Frequency}
				i := 0
				for {
					l, r := 2*i+1, 2*i+2
					if l >= leastExercisedInvariantCount {
						break
					}
					small := l
					if r < leastExercisedInvariantCount && h[r].count > h[l].count {
						small = r
					}
					if h[i].count >= h[small].count {
						break
					}
					h[i], h[small] = h[small], h[i]
					i = small
				}
			}
		}

		sort.Slice(h, func(i, j int) bool {
			return h[i].count < h[j].count
		})

		for _, v := range h {
			a := assertionTracker[v.key]
			fmt.Printf(
				"count=%-4d | %-*s | %-*s | %s\n",
				a.Frequency,
				longestKindWord, a.Kind,
				longestMessageLength, a.Message,
				v.key,
			)
		}
	}
}

// Always calls assertionFailureCallback if cond is false.
//
// Note 1: If you need Always(item == nil), use AlwaysNil(item) instead.
//
// Note 2: When deferring assertions, enclose them in a closure. Otherwise, cond
// is evaluated immediately.
//
//	// Correct: deferred assertion evaluates cond later
//	defer func() { invariant.Always(x > 0, "x must be positive") }()
//
//	// Incorrect: cond evaluated immediately
//	defer invariant.Always(x > 0, "x must be positive")
//
// Note 3: The analyzer tracks assertions by source line, so multiple assertions
// on the same line via loops are treated as a single assertion. However, its
// frequency counter increments on every evaluation. To avoid inflating
// frequency statistics, wrap the loop in a closure that returns a single bool,
// so the assertion is evaluated once for the aggregate condition.
//
//	// Aggregate check over a loop
//	invariant.Always(func() bool {
//	    for _, k := range keys {
//	        if k == forbidden {
//	            return false
//	        }
//	    }
//	    return true
//	}(), "No forbidden keys present")
//
//go:noinline
func Always(cond bool, msg string) {
	if cond {
		registerAssertion()
	} else {
		assertionFailureCallback(fmt.Sprintf("%s: %s\n", AssertionFailureMsgPrefix, msg))
	}
}

// Sometimes records that a condition was true at least once throughout the test
// run. It only has an effect in test environments; outside of tests it is a
// no-op.
//
// Technically, the term "invariant.Sometimes" is a misnomer. It's more accurately
// described as a property check. But I wanted to name this library "invariant"
// instead of "assert" or "property" for better grep-ability... maybe also
// because it sounds cool.
//
//go:noinline
func Sometimes(ok bool, msg string) {
	if !IsRunningUnderGoTest || !ok {
		return
	}
	registerAssertion()
}

// AlwaysNil calls assertionFailureCallback if x is NOT nil and prints the
// non-null object. Prefer this over Always(x == nil) so that the value of x can
// be logged.
//
//go:noinline
func AlwaysNil(x interface{}, msg string) {
	if x == nil {
		registerAssertion()
	} else {
		assertionFailureCallback(fmt.Sprintf("%s: expected nil. got %v. %s\n", AssertionFailureMsgPrefix, x, msg))
	}
}

// AlwaysErrIs calls assertionFailureCallback if actual is NOT one of the
// specified targets. Must provide at least one target. All targets must not be
// nil.
//
//go:noinline
func AlwaysErrIs(actual error, targets []error, msg string) {
	Always(len(targets) > 0, "invariant.AlwaysErrIs requires at least one target")
	for _, t := range targets {
		Always(t != nil, "All invariant.AlwaysErrIs targets must not be nil")
		if errors.Is(actual, t) {
			registerAssertion()
			return
		}
	}
	assertionFailureCallback(fmt.Sprintf("%s: error did not match any targets. got %q. %s\n", AssertionFailureMsgPrefix, actual, msg))
}

// AlwaysErrIsNot calls assertionFailureCallback if actual is one of the
// specified targets. Must provide at least one target. All targets must not be
// nil.
//
//go:noinline
func AlwaysErrIsNot(actual error, targets []error, msg string) {
	Always(len(targets) > 0, "invariant.AlwaysErrIsNot() must have at least one target")
	for _, t := range targets {
		Always(t != nil, "invariant.AlwaysErrIsNot() targets must not be nil")
		if errors.Is(actual, t) {
			assertionFailureCallback(fmt.Sprintf("%s: error unexpectedly matched a target. got %q. %s\n", AssertionFailureMsgPrefix, actual, msg))
			return
		}
	}
	registerAssertion()
}

type _Number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

// Until returns a bounded sequence useful for safely constraining infinite loops.
// If the break condition never evaluates, the loop panics on the final iteration.
// Under "disable_assertions", the loop still runs but runaway loops are undetected.
// If you need an explicit game loop, refer to invariant.GameLoop.
//
// Usage:
//
//	// Instead of this:
//	for {
//		if cond() {
//			break
//		}
//		doWork()
//	}
//	// Do this:
//	for range invariant.Until(10_000) {
//		if cond() {
//			break
//		}
//		doWork()
//	}
//
//go:noinline
func Until[T _Number](limit T) iter.Seq[int] {
	Always(limit > 0, "Loop bound is a positive integer")
	return func(yield func(int) bool) {
		limit := int(limit)
		for iteration := range limit {
			if iteration == limit-1 {
				assertionFailureCallback("Runaway loop!")
				return
			}
			if !yield(iteration) {
				return
			}
		}
	}
}

//go:noinline
func Unimplemented(msg string) {
	assertionFailureCallback(fmt.Sprintf("%s: %s\n", AssertionFailureMsgPrefix, msg))
}

//go:noinline
func Unreachable(msg string) {
	assertionFailureCallback(fmt.Sprintf("%s: %s\n", AssertionFailureMsgPrefix, msg))
}
