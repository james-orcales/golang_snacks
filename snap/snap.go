package snap

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

func New(data string) Snapshot {
	callers := [1]uintptr{}
	count := runtime.Callers(2, callers[:])
	frame, _ := runtime.CallersFrames(callers[:count]).Next()

	return Snapshot{
		Expect:   data,
		FilePath: frame.File,
		Line:     frame.Line,
	}
}

// Used for GO_SNAPSHOT_UPDATE_ALL
var lines_added = 0

// Diff returns true if actual == snapshot.Expect
func (snapshot Snapshot) Diff(actual string) (matched bool) {
	assert(strings.Count(snapshot.Expect, "`") == 0, "Snapshot expected value does not contain backticks")
	assert(strings.Count(actual, "`") == 0, "Snapshot actual value does not contain backticks")
	assert(snapshot.Line > 1, "Go source have package declaration or comments in the first line")
	assert(filepath.IsAbs(snapshot.FilePath), "Snapshot location is an absolute path")

	matched = actual == snapshot.Expect
	should_update := snapshot.ShouldUpdate || os.Getenv("SNAPSHOT_UPDATE_ALL") == "1"

	if !should_update {
		if !matched {
			fmt.Printf(`Snapshot differs
Expected:
---------
%s
---------
Actual:
---------
%s
---------
`, snapshot.Expect, actual)
		}
		return matched
	} else {
		snapshot.Line += lines_added
		defer func() {
			lines_added += strings.Count(actual, "\n") - strings.Count(snapshot.Expect, "\n")
		}()
		content, err := os.ReadFile(snapshot.FilePath)
		if err != nil {
			panic(fmt.Sprintf("Update snapshot | can't read file: %s\n", err))
		}
		line_count := 1
		start, end := -1, -1
		for i, b := range content {
			if b == '\n' {
				line_count++
				if line_count < snapshot.Line {
				} else if line_count == snapshot.Line {
					start = i + 1
				} else if line_count == snapshot.Line+1 {
					end = i
					break
				} else {
					assert(true, "Stopped parsing at the next line after snap.New")
				}
			}
		}
		assert(start >= 0 && end >= 0, "Snapshot is found")
		assert(start > 1, "Go source have package declaration or comments in the first line")
		assert(content[start-1] == '\n', "Line starts after newline")
		assert(content[end] == '\n', "Line ends with newline")

		line := string(content[start:end])
		assert(strings.Count(line, "snap.New") == 1, "Only one snap.New call per line")
		snap_New := strings.Index(string(content[start:end]), "snap.New")
		assert(snap_New >= 0, "Found snapshot in expected line")
		snap_New += start

		open, close := -1, -1
		for i, b := range content[snap_New:] {
			i += snap_New
			if b == '`' {
				if open < 0 {
					open = i
				} else {
					close = i
					break
				}
			}
		}
		assert(open >= 0, "Found open backtick")
		assert(close >= 0, "Found closed backtick")
		assert(open < close, "Open backtick comes before closed backtick")

		var write_err error
		find, replace := "`).Update()", "`)"
		if len(content[close:]) >= len(find) && string(content[close:][:len(find)]) == find {
			if snapshot.Expect == actual {
				fmt.Printf("Actual matches expected. Removing unnecessary .Update() at %s:%d", snapshot.FilePath, snapshot.Line)
			}
			write_err = os.WriteFile(
				snapshot.FilePath,
				bytes.Join([][]byte{content[:open+1], []byte(actual), []byte(replace), content[close+len(find):]}, nil),
				0o644,
			)
		} else {
			if matched {
				return true
			}
			write_err = os.WriteFile(
				snapshot.FilePath,
				bytes.Join([][]byte{content[:open+1], []byte(actual), content[close:]}, nil),
				0o644,
			)
		}
		if write_err != nil {
			panic(fmt.Sprintf("Couldn't commit snapshot update: %s:%d\n", snapshot.FilePath, snapshot.Line))
		}

		fmt.Printf("UPDATED SNAPSHOT %s:%d\n", snapshot.FilePath, snapshot.Line)
		return matched
	}
}

func (snapshot Snapshot) Update() Snapshot {
	snapshot.ShouldUpdate = true
	return snapshot
}

type Snapshot struct {
	Expect       string
	FilePath     string
	Line         int
	ShouldUpdate bool
}

func assert(cond bool, msg string) {
	if !cond {
		fprintStackTrace(os.Stderr, 2)
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
}

func fprintStackTrace(w io.Writer, callerLocation int) {
	const depth = 15
	var pcs [depth]uintptr
	skip := 1 + max(0, callerLocation)

	n := runtime.Callers(skip, pcs[:])
	fs := runtime.CallersFrames(pcs[:n])

	var frames [depth]runtime.Frame
	i := 0
	for {
		frame, ok := fs.Next()
		if !ok || i >= len(frames) {
			break
		}
		frame.Function = path.Base(frame.Function)
		frames[i] = frame
		i++
	}

	maxFn := 0
	for j := 0; j < i; j++ {
		n := len(frames[j].Function)
		if n > maxFn {
			maxFn = n
		}
	}

	for j := 0; j < i; j++ {
		frame := frames[j]
		if frame.File == "_testmain.go" {
			continue
		}
		switch frame.Function {
		case "runtime.main", "testing.tRunner":
			continue
		}
		fmt.Fprintf(w,
			"%-*s | %s:%d\n",
			maxFn,
			frame.Function,
			frame.File,
			frame.Line,
		)
	}
}
