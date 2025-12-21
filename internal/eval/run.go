package eval

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sys/unix"
)

// Runner executes execution plans.
type Runner struct {
	Env           *Env
	Builtins      map[string]Builtin
	Trace         bool
	TraceWriter   io.Writer
	Interactive   bool
	TTYFD         int
	ShellPgid     int
	ForegroundPgid int
	mu            sync.Mutex
	exitRequested bool
	exitCode      int
}

// ExitRequested reports whether an exit builtin has been invoked.
func (r *Runner) ExitRequested() bool {
	if r == nil {
		return false
	}
	return r.exitRequested
}

// ExitCode returns the requested exit code.
func (r *Runner) ExitCode() int {
	if r == nil {
		return 0
	}
	return r.exitCode
}

// Foreground returns the current foreground process group.
func (r *Runner) Foreground() int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ForegroundPgid
}

// Result captures the exit status.
type Result struct {
	Status int
}

// RunPlan executes a plan tree and returns the final status.
func (r *Runner) RunPlan(p *ExecPlan, stdin io.Reader, stdout, stderr io.Writer) Result {
	if r.Env == nil {
		r.Env = NewEnv(nil)
	}
	if r.Builtins == nil {
		r.Builtins = defaultBuiltins()
	}
	if r.Trace && r.TraceWriter == nil {
		r.TraceWriter = io.Discard
	}
	if r.Interactive && r.ShellPgid == 0 {
		r.ShellPgid = unix.Getpgrp()
	}
	status := r.runChain(p, stdin, stdout, stderr)
	return Result{Status: status}
}

func (r *Runner) runChain(p *ExecPlan, stdin io.Reader, stdout, stderr io.Writer) int {
	status := 0
	for cur := p; cur != nil; cur = cur.Next {
		if r.exitRequested {
			return r.exitCode
		}
		if cur.Background {
			go r.runSingle(cur, stdin, stdout, stderr)
			status = 0
			r.Env.SetStatus(status)
			continue
		}
		status = r.runSingle(cur, stdin, stdout, stderr)
		r.Env.SetStatus(status)
		if r.exitRequested {
			return r.exitCode
		}
		if status == 0 && cur.IfOK != nil {
			status = r.runChain(cur.IfOK, stdin, stdout, stderr)
			r.Env.SetStatus(status)
		}
		if status != 0 && cur.IfFail != nil {
			status = r.runChain(cur.IfFail, stdin, stdout, stderr)
			r.Env.SetStatus(status)
		}
	}
	return status
}

func (r *Runner) runSingle(p *ExecPlan, stdin io.Reader, stdout, stderr io.Writer) int {
	if p == nil {
		return 0
	}
	if p.PipeTo != nil {
		return r.runPipe(p, p.PipeTo, stdin, stdout, stderr)
	}
	return r.runStage(p, stdin, stdout, stderr)
}

func (r *Runner) runPipe(left, right *ExecPlan, stdin io.Reader, stdout, stderr io.Writer) int {
	pr, pw := io.Pipe()
	leftDone := make(chan int, 1)
	rightDone := make(chan int, 1)

	go func() {
		leftDone <- r.runStage(left, stdin, pw, stderr)
		_ = pw.Close()
	}()
	go func() {
		rightDone <- r.runStage(right, pr, stdout, stderr)
		_ = pr.Close()
	}()

	_ = <-leftDone
	status := <-rightDone
	return status
}

func (r *Runner) runStage(p *ExecPlan, stdin io.Reader, stdout, stderr io.Writer) int {
	if p == nil {
		return 0
	}
	if p.Kind == PlanFnDef {
		if p.Func != nil && p.Func.Name != "" {
			r.Env.SetFunc(p.Func.Name, p.Func.Body)
		}
		return 0
	}
	if p.Kind == PlanAssign {
		vals, err := ExpandValue(p.AssignVal, r.Env)
		if err != nil {
			return 1
		}
		r.Env.Set(p.AssignName, vals)
		return 0
	}
	execEnv := r.Env
	if len(p.Prefix) > 0 {
		child := NewChild(r.Env)
		for _, pref := range p.Prefix {
			vals, err := ExpandValue(pref.Val, child)
			if err != nil {
				return 1
			}
			child.Set(pref.Name, vals)
		}
		execEnv = child
	}
	argv, err := r.expandArgv(p, execEnv)
	if err != nil {
		return 1
	}
	if len(argv) == 0 {
		return 0
	}
	r.tracef("+ %s\n", strings.Join(argv, " "))
	if def, ok := execEnv.GetFunc(argv[0]); ok {
		return r.runFuncCall(def, argv, p, execEnv, stdin, stdout, stderr)
	}
	if builtin, ok := r.Builtins[argv[0]]; ok {
		return r.runBuiltin(builtin, argv, p, execEnv, stdin, stdout, stderr)
	}
	return r.runExternal(argv, p, stdin, stdout, stderr)
}

func (r *Runner) runBuiltin(builtin Builtin, argv []string, p *ExecPlan, env *Env, stdin io.Reader, stdout, stderr io.Writer) int {
	in := stdin
	out := stdout
	files, err := applyRedirs(p, &in, &out)
	if err != nil {
		return 1
	}
	for _, f := range files {
		defer f.Close()
	}
	orig := r.Env
	r.Env = env
	status := builtin(in, out, stderr, argv, r)
	r.Env = orig
	return status
}

func (r *Runner) runExternal(argv []string, p *ExecPlan, stdin io.Reader, stdout, stderr io.Writer) int {
	cmd, cleanup, err := buildCmd(argv, p, stdin, stdout, stderr)
	if err != nil {
		return 127
	}
	if cmd == nil {
		return 0
	}
	defer cleanup()
	cmd.SysProcAttr = &unix.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return exitStatus(err)
	}
	r.attachForeground(cmd.Process.Pid)
	err = cmd.Wait()
	r.restoreForeground()
	return exitStatus(err)
}

func (r *Runner) runFuncCall(def FuncDef, argv []string, p *ExecPlan, env *Env, stdin io.Reader, stdout, stderr io.Writer) int {
	in := stdin
	out := stdout
	files, err := applyRedirs(p, &in, &out)
	if err != nil {
		return 1
	}
	for _, f := range files {
		defer f.Close()
	}
	child := NewChild(env)
	args := []string{}
	if len(argv) > 1 {
		args = argv[1:]
	}
	child.Set("*", args)
	child.Set("0", []string{argv[0]})
	for i, arg := range args {
		child.Set(strconv.Itoa(i+1), []string{arg})
	}
	bodyPlan, err := BuildPlan(def.Body, child)
	if err != nil {
		return 1
	}
	origEnv := r.Env
	r.Env = child
	status := r.runChain(bodyPlan, in, out, stderr)
	r.Env = origEnv
	return status
}

func buildCmd(argv []string, p *ExecPlan, stdin io.Reader, stdout, stderr io.Writer) (*exec.Cmd, func(), error) {
	if p == nil || len(argv) == 0 {
		return nil, func() {}, nil
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	files, err := applyRedirs(p, &cmd.Stdin, &cmd.Stdout)
	cleanup := func() {
		for _, f := range files {
			_ = f.Close()
		}
	}
	if err != nil {
		cleanup()
		return nil, func() {}, err
	}
	return cmd, cleanup, nil
}

func (r *Runner) expandArgv(p *ExecPlan, env *Env) ([]string, error) {
	if p == nil {
		return nil, nil
	}
	if p.Call == nil {
		return p.Argv, nil
	}
	return ExpandCall(p.Call, env)
}

func (r *Runner) attachForeground(pid int) {
	if !r.Interactive || r.TTYFD <= 0 {
		return
	}
	pgid, err := unix.Getpgid(pid)
	if err != nil {
		return
	}
	r.mu.Lock()
	r.ForegroundPgid = pgid
	r.mu.Unlock()
	_ = unix.IoctlSetInt(r.TTYFD, unix.TIOCSPGRP, pgid)
}

func (r *Runner) restoreForeground() {
	if !r.Interactive || r.TTYFD <= 0 {
		r.mu.Lock()
		r.ForegroundPgid = 0
		r.mu.Unlock()
		return
	}
	if r.ShellPgid != 0 {
		_ = unix.IoctlSetInt(r.TTYFD, unix.TIOCSPGRP, r.ShellPgid)
	}
	r.mu.Lock()
	r.ForegroundPgid = 0
	r.mu.Unlock()
}

func (r *Runner) tracef(format string, args ...any) {
	if !r.Trace || r.TraceWriter == nil {
		return
	}
	fmt.Fprintf(r.TraceWriter, format, args...)
}

func applyRedirs(p *ExecPlan, stdin *io.Reader, stdout *io.Writer) ([]*os.File, error) {
	if p == nil {
		return nil, nil
	}
	var files []*os.File
	for _, r := range p.Redirs {
		if len(r.Target) == 0 {
			continue
		}
		path := r.Target[0]
		switch r.Op {
		case ">":
			f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o666)
			if err != nil {
				return files, err
			}
			*stdout = f
			files = append(files, f)
		case ">>":
			f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o666)
			if err != nil {
				return files, err
			}
			*stdout = f
			files = append(files, f)
		case "<":
			f, err := os.Open(path)
			if err != nil {
				return files, err
			}
			*stdin = f
			files = append(files, f)
		case "<>":
			f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o666)
			if err != nil {
				return files, err
			}
			*stdin = f
			files = append(files, f)
		}
	}
	return files, nil
}

func exitStatus(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return 127
}
