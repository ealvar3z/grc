package eval

import (
	"io"
	"os"
	"os/exec"
)

// Runner executes execution plans.
type Runner struct {
	Env           *Env
	Builtins      map[string]Builtin
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
	if len(p.Argv) == 0 {
		return 0
	}
	if builtin, ok := r.Builtins[p.Argv[0]]; ok {
		return r.runBuiltin(builtin, p, stdin, stdout, stderr)
	}
	return r.runExternal(p, stdin, stdout, stderr)
}

func (r *Runner) runBuiltin(builtin Builtin, p *ExecPlan, stdin io.Reader, stdout, stderr io.Writer) int {
	in := stdin
	out := stdout
	files, err := applyRedirs(p, &in, &out)
	if err != nil {
		return 1
	}
	for _, f := range files {
		defer f.Close()
	}
	return builtin(in, out, stderr, p.Argv, r)
}

func (r *Runner) runExternal(p *ExecPlan, stdin io.Reader, stdout, stderr io.Writer) int {
	cmd, cleanup, err := buildCmd(p, stdin, stdout, stderr)
	if err != nil {
		return 127
	}
	if cmd == nil {
		return 0
	}
	defer cleanup()
	if err := cmd.Start(); err != nil {
		return exitStatus(err)
	}
	err = cmd.Wait()
	return exitStatus(err)
}

func buildCmd(p *ExecPlan, stdin io.Reader, stdout, stderr io.Writer) (*exec.Cmd, func(), error) {
	if p == nil || len(p.Argv) == 0 {
		return nil, func() {}, nil
	}
	cmd := exec.Command(p.Argv[0], p.Argv[1:]...)
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
