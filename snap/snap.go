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
	"sync"

	"github.com/james-orcales/golang_snacks/myers"
)

const (
	GLOBAL_EDIT_ENV        = "SNAPSHOT_EDIT_ALL"
	GLOBAL_EDIT_ENV_ENABLE = "1"
)

type Snapshot struct {
	Expect     string
	FilePath   string
	Line       int
	ShouldEdit bool
}

// WARN: Brittle under go:generate
func Init(data string) Snapshot {
	callers := [1]uintptr{}
	count := runtime.Callers(2, callers[:])
	frame, _ := runtime.CallersFrames(callers[:count]).Next()

	return Snapshot{
		Expect:   data,
		FilePath: frame.File,
		Line:     frame.Line,
	}
}

func Edit(data string) Snapshot {
	callers := [1]uintptr{}
	count := runtime.Callers(2, callers[:])
	frame, _ := runtime.CallersFrames(callers[:count]).Next()

	return Snapshot{
		Expect:     data,
		FilePath:   frame.File,
		Line:       frame.Line,
		ShouldEdit: true,
	}
}

type FileEdit struct {
	Line, Delta int
}

var filesEdited = make(map[string][]FileEdit)
var filesEditedMu = sync.Mutex{}

func (snapshot Snapshot) IsEqual(actual string) (isEqual bool) {
	assert(strings.Count(snapshot.Expect, "`") == 0, "Snapshot expected value does not contain backticks")
	assert(strings.Count(actual, "`") == 0, "Snapshot actual value does not contain backticks")
	assert(snapshot.Line > 1, "Go source have package declaration or comments in the first line")
	assert(filepath.IsAbs(snapshot.FilePath), "Snapshot location is an absolute path")

	compileTimeLine := snapshot.Line
	isEqual = actual == snapshot.Expect
	if snapshot.ShouldEdit || os.Getenv(GLOBAL_EDIT_ENV) == GLOBAL_EDIT_ENV_ENABLE {
		filesEditedMu.Lock()
		defer filesEditedMu.Unlock()

		if edits, ok := filesEdited[snapshot.FilePath]; ok {
			offset := 0
			for _, edit := range edits {
				if edit.Line < compileTimeLine {
					offset += edit.Delta
				}
			}
			snapshot.Line += offset
		}

		content, err := os.ReadFile(snapshot.FilePath)
		if err != nil {
			panic(fmt.Sprintf("Update snapshot | can't read file: %s\n", err))
		}

		lineCount := 1
		start, end := -1, -1
		for i, b := range content {
			if b == '\n' {
				lineCount++
				if lineCount < snapshot.Line {
					continue
				} else if lineCount == snapshot.Line {
					start = i + 1
				} else if lineCount == snapshot.Line+1 {
					end = i
					break
				} else {
					assert(true, "Stopped parsing at the next line after snap.Init")
				}
			}
		}
		assert(start >= 0 && end >= 0, "Snapshot is found")
		assert(start > 1, "Go source have package declaration or comments in the first line")
		assert(content[start-1] == '\n', "Line starts after newline")
		assert(content[end] == '\n', "Line ends with newline")

		line := string(content[start:end])
		replace := "snap.Init(`"
		search := "snap.Init(`"
		if snapshot.ShouldEdit {
			search = "snap.Edit(`"
		}
		assert(len(search) == len(replace), "Find and replace strings are of equal length")
		assert(strings.Count(line, search) > 0, "Found snapshot in expected line")
		assert(strings.Count(line, search) == 1, "Only one snap call per line")

		snap_call_idx := strings.Index(line, search)
		assert(snap_call_idx >= 0, "Found snapshot call")
		snap_call_idx += start

		open, close := snap_call_idx+len(search)-1, -1
		for i, b := range content[open+1:] {
			if b == '`' {
				close = i + open + 1
				break
			}
		}
		assert(open >= 0, "Found open backtick")
		assert(close >= 0, "Found closed backtick")
		assert(open < close, "Open backtick comes before closed backtick")

		new_content := &bytes.Buffer{}
		new_content.Grow(len(content))
		new_content.Write(content[:open+1-len(search)])
		new_content.WriteString(replace)
		new_content.WriteString(actual)
		new_content.Write(content[close:])

		if err := os.WriteFile(snapshot.FilePath, new_content.Bytes(), 0o664); err != nil {
			panic(err)
		}

		if !isEqual {
			delta := strings.Count(actual, "\n") - strings.Count(snapshot.Expect, "\n")
			if _, ok := filesEdited[snapshot.FilePath]; !ok {
				filesEdited[snapshot.FilePath] = make([]FileEdit, 0)
			}
			filesEdited[snapshot.FilePath] = append(filesEdited[snapshot.FilePath], FileEdit{Line: compileTimeLine, Delta: delta})
		}

		fmt.Printf("UPDATED SNAPSHOT %s:%d\n", snapshot.FilePath, snapshot.Line)
		return true
	} else {
		if !isEqual {
			d := myers.New(snapshot.Expect, actual)
			fmt.Fprintf(os.Stderr, "Snapshot mismatch %s:%d\n", snapshot.FilePath, snapshot.Line)
			for line := range strings.SplitSeq(d.LineDiff(), "\n") {
				if len(line) == 0 {
					fmt.Println(line)
					continue
				}
				switch line[0] {
				case '+':
					fmt.Println("\033[32m" + line + "\033[0m")
				case '-':
					fmt.Println("\033[31m" + line + "\033[0m")
				default:
					fmt.Println(line)
				}
			}
		}
		return isEqual
	}
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
