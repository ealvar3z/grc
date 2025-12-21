package eval

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// Builtin executes a built-in command and returns an exit status.
type Builtin func(stdin io.Reader, stdout, stderr io.Writer, args []string, r *Runner) int

func defaultBuiltins() map[string]Builtin {
	return map[string]Builtin{
		"apid": builtinAPID,
		"bg":   builtinBG,
		"cd":   builtinCD,
		"fg":   builtinFG,
		"jobs": builtinJobs,
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

func builtinJobs(stdin io.Reader, stdout, stderr io.Writer, args []string, r *Runner) int {
	_ = stdin
	_ = stderr
	_ = args
	if r == nil {
		return 0
	}
	jobs := r.listJobs()
	if len(jobs) == 0 {
		return 0
	}
	_, _ = fmt.Fprint(stdout, formatJobs(jobs))
	r.mu.Lock()
	for _, job := range jobs {
		if job.State == "done" {
			job.Notified = true
		}
	}
	r.mu.Unlock()
	r.pruneJobs()
	return 0
}

func builtinAPID(stdin io.Reader, stdout, stderr io.Writer, args []string, r *Runner) int {
	_ = stdin
	_ = stderr
	_ = args
	if r == nil || r.Env == nil {
		return 0
	}
	vals := r.Env.Get("apid")
	if len(vals) == 0 {
		return 0
	}
	fmt.Fprintln(stdout, strings.Join(vals, " "))
	return 0
}

func builtinFG(stdin io.Reader, stdout, stderr io.Writer, args []string, r *Runner) int {
	_ = stdin
	_ = stdout
	if r == nil {
		return 1
	}
	job, err := resolveJob(args, r)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if job.State == "done" {
		r.removeJob(job.ID)
		return job.Exit
	}
	_ = unix.Kill(-job.Pgid, unix.SIGCONT)
	r.attachForegroundPgid(job.Pgid)
	exit := r.waitJob(job)
	r.restoreForeground()
	r.removeJob(job.ID)
	return exit
}

func builtinBG(stdin io.Reader, stdout, stderr io.Writer, args []string, r *Runner) int {
	_ = stdin
	_ = stdout
	if r == nil {
		return 1
	}
	job, err := resolveJob(args, r)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	_ = unix.Kill(-job.Pgid, unix.SIGCONT)
	job.State = "running"
	return 0
}

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

func resolveJob(args []string, r *Runner) (*Job, error) {
	if r == nil {
		return nil, fmt.Errorf("no runner")
	}
	var job *Job
	if len(args) > 1 {
		id := parseJobID(args[1])
		if id == 0 {
			return nil, fmt.Errorf("invalid job id")
		}
		job = r.getJob(id)
	} else {
		job = r.lastJob()
	}
	if job == nil {
		return nil, fmt.Errorf("no jobs")
	}
	return job, nil
}

func parseJobID(s string) int {
	if strings.HasPrefix(s, "%") {
		s = strings.TrimPrefix(s, "%")
	}
	id, err := parseInt(s)
	if err != nil || id <= 0 {
		return 0
	}
	return id
}
