package eval

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"grc/internal/parse"
)

// Builtin executes a built-in command and returns an exit status.
type Builtin func(stdin io.Reader, stdout, stderr io.Writer, args []string, r *Runner) int

func defaultBuiltins() map[string]Builtin {
	return map[string]Builtin{
		"apid":    builtinAPID,
		"bg":      builtinBG,
		"cd":      builtinCD,
		".":       builtinDot,
		"exec":    builtinExec,
		"fg":      builtinFG,
		"jobs":    builtinJobs,
		"newpgrp": builtinNewpgrp,
		"pwd":     builtinPWD,
		"exit":    builtinExit,
		"eval":    builtinEval,
		"which":   builtinWhich,
		"shift":   builtinShift,
		"return":  builtinReturn,
		"wait":    builtinWait,
	}
}

// BuiltinNames returns the default builtin command names.
func BuiltinNames() []string {
	names := make([]string, 0, len(defaultBuiltins()))
	for name := range defaultBuiltins() {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
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
	path, ok := resolvePath(argv[0], r.Env, true, stderr)
	if !ok {
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

func builtinEval(stdin io.Reader, stdout, stderr io.Writer, args []string, r *Runner) int {
	if len(args) < 2 || r == nil || r.Env == nil {
		return 0
	}
	src := strings.Join(args[1:], " ")
	ast, err := parse.ParseAll(strings.NewReader(src))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	plan, err := BuildPlan(ast, r.Env)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return r.runChain(plan, stdin, stdout, stderr)
}

func builtinWhich(stdin io.Reader, stdout, stderr io.Writer, args []string, r *Runner) int {
	_ = stdin
	if len(args) < 2 {
		return 0
	}
	ok := true
	for _, name := range args[1:] {
		path, found := resolvePath(name, r.Env, true, stderr)
		if !found {
			ok = false
			continue
		}
		fmt.Fprintln(stdout, path)
	}
	if ok {
		return 0
	}
	return 1
}

func builtinNewpgrp(stdin io.Reader, stdout, stderr io.Writer, args []string, r *Runner) int {
	_ = stdin
	_ = stdout
	if len(args) > 1 {
		fmt.Fprintln(stderr, "newpgrp: too many arguments")
		return 1
	}
	pid := os.Getpid()
	if err := syscall.Setpgid(0, 0); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if r != nil {
		r.ShellPgid = pid
	}
	_ = unix.IoctlSetPointerInt(2, unix.TIOCSPGRP, pid)
	return 0
}

func builtinShift(stdin io.Reader, stdout, stderr io.Writer, args []string, r *Runner) int {
	_ = stdin
	_ = stdout
	if r == nil || r.Env == nil {
		return 1
	}
	shift := 1
	if len(args) > 2 {
		fmt.Fprintln(stderr, "shift: too many arguments")
		return 1
	}
	if len(args) == 2 {
		n, err := parseInt(args[1])
		if err != nil || n < 0 {
			fmt.Fprintln(stderr, "shift: bad shift count")
			return 1
		}
		shift = n
	}
	argsList := r.Env.Get("*")
	if shift > len(argsList) {
		fmt.Fprintln(stderr, "rc: cannot shift")
		return 1
	}
	argsList = argsList[shift:]
	r.Env.Set("*", argsList)
	r.Env.SetPositional(argsList)
	return 0
}

func builtinReturn(stdin io.Reader, stdout, stderr io.Writer, args []string, r *Runner) int {
	_ = stdin
	_ = stdout
	_ = stderr
	code := 0
	if len(args) > 2 {
		fmt.Fprintln(stderr, "return: too many arguments")
		return 1
	}
	if len(args) == 2 {
		if n, err := parseInt(args[1]); err == nil {
			code = n
		}
	}
	if r != nil {
		r.requestReturn(code)
	}
	return code
}

func builtinWait(stdin io.Reader, stdout, stderr io.Writer, args []string, r *Runner) int {
	_ = stdin
	_ = stdout
	if r == nil {
		return 1
	}
	if len(args) == 1 {
		jobs := r.listJobs()
		if len(jobs) == 0 {
			return 0
		}
		status := 0
		for _, job := range jobs {
			status = r.waitJob(job)
		}
		return status
	}
	status := 0
	for _, a := range args[1:] {
		pid, err := parseInt(a)
		if err != nil || pid <= 0 {
			fmt.Fprintf(stderr, "rc: `%s' is a bad number\n", a)
			return 1
		}
		job := r.findJobByPID(pid)
		if job == nil {
			fmt.Fprintf(stderr, "rc: `%s' is not a child\n", a)
			return 1
		}
		status = r.waitJob(job)
	}
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
