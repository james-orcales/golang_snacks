package snap

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"strings"

	"golang_snacks/invariant"
)

func New(data string) Snapshot {
	callers := [1]uintptr{}
	count := runtime.Callers(2, callers[:])
	frame, _ := runtime.CallersFrames(callers[:count]).Next()

	return Snapshot{
		Data:     data,
		FilePath: frame.File,
		Line:     frame.Line,
	}
}

// Diff returns true if actual == snapshot.Data
func (snapshot Snapshot) Diff(actual string) bool {
	if snapshot.Data != actual && !snapshot.ShouldUpdate {
		defer fmt.Printf(`Snapshot differs
Expected:
---------
%s
---------
Actual:
---------
%s
---------
`, snapshot.Data, actual)
	}
	// Update file even when data matches so that the unnecessary Update() is removed
	if snapshot.ShouldUpdate || os.Getenv("GO_SNAPSHOT_UPDATE_ALL") != "" {
		defer fmt.Println("UPDATED SNAPSHOT")
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
					invariant.Unreachable("Stopped parsing at the very next line")
				}
			}
		}
		if start < 0 || end < 0 {
			panic(fmt.Sprintf("Couldn't find index snapshot line in file %q\n", snapshot.FilePath))
		}
		invariant.Always(start > 1, "Go source have package declaration or comments in the first line")

		line := string(content[start:end])
		invariant.Always(strings.Count(line, "snap.New") == 1, "Only one snap.New call per line")
		snap_New := strings.Index(string(content[start:end]), "snap.New")
		if snap_New < 0 {
			panic(fmt.Sprintf("Couldn't find snapshot in %s:%d\n", snapshot.FilePath, snapshot.Line))
		}
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
		if open < 0 || close < 0 || open == close || open > close {
			panic(fmt.Sprintf("Couldn't find backtick pair to replace snapshot: %s:%d\n", snapshot.FilePath, snapshot.Line))
		}
		var write_err error
		for retry := 0; retry < 10; retry++ {
			write_err = os.WriteFile(
				snapshot.FilePath,
				bytes.Join([][]byte{content[:open+1], []byte(actual), []byte("`)"), content[close+len(").Update()")+1:]}, nil),
				0o644,
			)
			if write_err == nil {
				break
			}
		}
		if write_err != nil {
			panic(fmt.Sprintf("Couldn't commit snapshot update: %s:%d\n", snapshot.FilePath, snapshot.Line))
		}
	}
	return actual == snapshot.Data
}

func (snapshot Snapshot) Update() Snapshot {
	snapshot.ShouldUpdate = true
	return snapshot
}

type Snapshot struct {
	Data         string
	FilePath     string
	Line         int
	ShouldUpdate bool
}
