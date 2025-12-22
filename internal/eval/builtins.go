package eval

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"grc/internal/parse"
)

// Builtin executes a built-in command and returns an exit status.
type Builtin func(stdin io.Reader, stdout, stderr io.Writer, args []string, r *Runner) int

func defaultBuiltins() map[string]Builtin {
	return map[string]Builtin{
		"apid": builtinAPID,
		"bg":   builtinBG,
		"cd":   builtinCD,
		".":    builtinDot,
		"exec": builtinExec,
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

func builtinExec(stdin io.Reader, stdout, stderr io.Writer, args []string, r *Runner) int {
	_ = stdin
	_ = stdout
	if len(args) < 2 {
		return 0
	}
	argv := args[1:]
	path, err := lookupPath(argv[0], r.Env)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 127
	}
	envList := buildExecEnv(r.Env)
	if err := syscall.Exec(path, argv, envList); err != nil {
		fmt.Fprintln(stderr, err)
		return 127
	}
	return 0
}

func builtinDot(stdin io.Reader, stdout, stderr io.Writer, args []string, r *Runner) int {
	if len(args) < 2 || r == nil || r.Env == nil {
		return 0
	}
	i := 1
	interactive := false
	if args[i] == "-i" {
		interactive = true
		i++
		if i >= len(args) {
			return 0
		}
	}
	path := args[i]
	rest := []string{}
	if i+1 < len(args) {
		rest = args[i+1:]
	}
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer f.Close()

	oldStar, hadStar := r.Env.GetLocal("*")
	oldZero, hadZero := r.Env.GetLocal("0")
	r.Env.Set("*", rest)
	r.Env.Set("0", []string{path})

	oldInteractive := r.Interactive
	if interactive {
		r.Interactive = true
	}
	ast, err := parse.ParseAll(f)
	if err != nil {
		fmt.Fprintln(stderr, err)
		r.Interactive = oldInteractive
		restoreVar(r.Env, "*", oldStar, hadStar)
		restoreVar(r.Env, "0", oldZero, hadZero)
		return 1
	}
	plan, err := BuildPlan(ast, r.Env)
	if err != nil {
		fmt.Fprintln(stderr, err)
		r.Interactive = oldInteractive
		restoreVar(r.Env, "*", oldStar, hadStar)
		restoreVar(r.Env, "0", oldZero, hadZero)
		return 1
	}
	status := r.runChain(plan, stdin, stdout, stderr)
	r.Interactive = oldInteractive
	restoreVar(r.Env, "*", oldStar, hadStar)
	restoreVar(r.Env, "0", oldZero, hadZero)
	return status
}

func restoreVar(env *Env, name string, val []string, had bool) {
	if env == nil {
		return
	}
	if had {
		env.Set(name, val)
	} else {
		env.Unset(name)
	}
}

func lookupPath(name string, env *Env) (string, error) {
	if strings.ContainsRune(name, '/') {
		return name, nil
	}
	pathList := []string{}
	if env != nil {
		if vals := env.Get("path"); len(vals) > 0 {
			pathList = vals
		}
	}
	if len(pathList) == 0 {
		pathList = strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))
	}
	for _, dir := range pathList {
		if dir == "" {
			dir = "."
		}
		full := filepath.Join(dir, name)
		info, err := os.Stat(full)
		if err != nil {
			continue
		}
		if info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
			return full, nil
		}
	}
	return "", fmt.Errorf("%s: not found", name)
}

func buildExecEnv(env *Env) []string {
	base := map[string]string{}
	for _, kv := range os.Environ() {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		base[parts[0]] = parts[1]
	}
	if env != nil {
		for k, v := range env.Snapshot() {
			base[k] = strings.Join(v, " ")
		}
	}
	out := make([]string, 0, len(base))
	for k, v := range base {
		out = append(out, k+"="+v)
	}
	return out
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
