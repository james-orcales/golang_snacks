package sh

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Spawn(arguments ...string) bool {
	return SpawnRaw("", os.Stdout, os.Stderr, arguments...)
}

func Pipe(arguments ...string) string {
	var out *bytes.Buffer
	Fpipe(out, arguments...)
	return out.String()
}

func PipeWithErr(arguments ...string) (string, error) {
	var out *bytes.Buffer
	var err *bytes.Buffer
	FpipeWithErr(out, err, arguments...)
	return out.String(), errors.New(err.String())
}

func Fpipe(out io.Writer, arguments ...string) bool {
	return FpipeWithErr(out, os.Stderr, arguments...)
}

func FpipeWithErr(out, err io.Writer, arguments ...string) bool {
	return SpawnRaw("", out, err, arguments...)
}

// SpawnRaw starts a process, treating leading ARG=VAL entries as env vars.
//
// Usage:
//
//	SpawnRaw("", nil, nil, "A=1", "B=2", "cmd")
//	SpawnRaw("", nil, nil, "A=1", "cmd", "x", "y")
//	SpawnRaw("", nil, nil, "cmd")
//	SpawnRaw("", nil, nil, "cmd", "x")
//
// The first non-ENV token becomes the executable. Remaining tokens become args.
// Returns true on zero exit status.
func SpawnRaw(workingDirectory string, out, err io.Writer, arguments ...string) bool {
	if len(arguments) == 0 {
		panic("scripting.spawn requires at least one argument")
	}
	if out == nil {
		out = os.Stdout
	}
	if err == nil {
		err = os.Stderr
	}
	var env []string
	for _, arg := range arguments {
		if strings.Contains(arg, "=") {
			env = append(env, arg)
		} else {
			break
		}
	}
	if len(arguments)-len(env) == 0 {
		panic("scripting.spawn was not provided an executable")
	}

	executable := arguments[0]
	if len(env) > 0 {
		executable = arguments[len(env)]
	}

	if len(arguments[len(env):]) > 1 {
		arguments = arguments[len(env)+1:]
	} else {
		arguments = nil
	}

	cmd := exec.Command(executable, arguments...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, env...)

	cmd.Stdout = out
	cmd.Stderr = err
	cmd.Stdin = os.Stdin

	if workingDirectory != "" {
		os.MkdirAll(workingDirectory, 0o755)
		cmd.Dir = workingDirectory
	}
	errCode := cmd.Run()
	return errCode == nil
}

// PushDir changes the current working directory to the specified dir and returns a function that
// restores the previous working directory. Call the returned function to revert to the original
// directory.
//
// Usage:
//
//	popDir := sh.PushDir("tmp")
//	doSomething()
//	popDir()
func PushDir(dir string) (popDir func()) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	if err := os.Chdir(dir); err != nil {
		panic(err)
	}
	return func() {
		if err := os.Chdir(workingDirectory); err != nil {
			panic(err)
		}
	}
}

func Which(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	path = filepath.Clean(path)
	return path
}

func MakeDirAll(path string) error {
	return os.MkdirAll(path, 0o755)
}

func Lines(s string, n int) []string {
	if n == 0 {
		return nil
	}
	if n > 0 {
		return strings.SplitN(s, "\n", n)[:n]
	} else {
		result := strings.Split(s, "\n")
		if len(result) > n {
			return result[len(result)-n:]
		} else {
			return result
		}
	}
}
