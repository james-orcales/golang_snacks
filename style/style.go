package style

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/james-orcales/golang_snacks/invariant"
)

func TestAll(t *testing.T) {
	LintAmbiguousLoopTermination(t)
}

func LintAmbiguousLoopTermination(t *testing.T) {
	// Relative to the calling package's directory.
	files := findGoFiles(".")
	invariant.Ensure(len(files) > 0, "Directory to find Go files in has at least one Go file")
	loops := findAmbiguousLoopTerminations(files)
	n := 0
	for _, lines := range loops {
		n += len(lines)
	}
	if n > 0 {
		t.Errorf("Detected %d ambiguously terminated loops. Replace with invariant.Until or invariant.GameLoop\n", n)
		for file, lines := range loops {
			for _, line := range lines {
				t.Errorf("\t%s:%d\n", file, line)
			}
		}
	}
}

func findAmbiguousLoopTerminations(files []string) map[string][]int {
	var waitGroup sync.WaitGroup

	locations := make(map[string][]int)
	for _, file := range files {
		invariant.Ensure(filepath.Ext(file) == ".go", "File to parse is Go source code")
		waitGroup.Add(1)

		go func(file string) {
			defer waitGroup.Done()

			parsedFileSet := token.NewFileSet()
			parsedFileAst, parseError := parser.ParseFile(parsedFileSet, file, nil, 0)
			if parseError != nil {
				panic(parseError)
			}

			ast.Inspect(parsedFileAst, func(astNode ast.Node) bool {
				// Skip benchmarks since assertions are disabled under them anyway
				if fd, ok := astNode.(*ast.FuncDecl); ok && strings.HasPrefix(fd.Name.Name, "Benchmark") {
					return false
				}

				if forStmt, ok := astNode.(*ast.ForStmt); ok {
					isSimpleInfiniteLoop := forStmt.Cond == nil || forStmt.Post == nil
					if isSimpleInfiniteLoop {
						position := parsedFileSet.Position(forStmt.Pos())
						locations[file] = append(locations[file], position.Line)
					}
				}
				return true
			})
		}(file)
	}
	waitGroup.Wait()

	return locations
}

func findGoFiles(dir string) []string {
	dir, err := filepath.Abs(dir)
	if err != nil {
		panic(err)
	}
	var files []string
	filepath.WalkDir(dir, func(file string, directoryEntry fs.DirEntry, err error) error {
		if err != nil {
			panic(err)
		}

		if !directoryEntry.IsDir() && filepath.Ext(file) == ".go" {
			files = append(files, file)
		}

		return nil
	})

	return files
}
