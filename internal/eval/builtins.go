package eval

import (
	"fmt"
	"io"
	"os"
)

// Builtin executes a built-in command and returns an exit status.
type Builtin func(stdin io.Reader, stdout, stderr io.Writer, args []string, r *Runner) int

func defaultBuiltins() map[string]Builtin {
	return map[string]Builtin{
		"cd":   builtinCD,
		"pwd":  builtinPWD,
		"exit": builtinExit,
	}
}

func builtinCD(stdin io.Reader, stdout, stderr io.Writer, args []string, r *Runner) int {
	_ = stdin
	_ = stdout
	var dir string
	if len(args) > 1 {
		dir = args[1]
	} else {
		if r != nil && r.Env != nil {
			if home := r.Env.Get("home"); len(home) > 0 {
				dir = home[0]
			}
		}
		if dir == "" {
			h, err := os.UserHomeDir()
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			dir = h
		}
	}
	if err := os.Chdir(dir); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func builtinPWD(stdin io.Reader, stdout, stderr io.Writer, args []string, r *Runner) int {
	_ = stdin
	_ = args
	_ = r
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, cwd)
	return 0
}

func builtinExit(stdin io.Reader, stdout, stderr io.Writer, args []string, r *Runner) int {
	_ = stdin
	_ = stdout
	_ = stderr
	code := 0
	if len(args) > 1 {
		if n, err := parseInt(args[1]); err == nil {
			code = n
		}
	}
	if r != nil {
		r.exitRequested = true
		r.exitCode = code
	}
	return code
}

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
