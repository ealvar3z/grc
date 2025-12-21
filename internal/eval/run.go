package eval

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"

	"grc/internal/parse"
)

// Runner executes execution plans.
type Runner struct {
	Env            *Env
	Builtins       map[string]Builtin
	Trace          bool
	TraceWriter    io.Writer
	Interactive    bool
	TTYFD          int
	ShellPgid      int
	ForegroundPgid int
	mu             sync.Mutex
	Jobs           map[int]*Job
	nextJobID      int
	exitRequested  bool
	exitCode       int
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
	if r.Jobs == nil {
		r.Jobs = make(map[int]*Job)
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

// CallFunc invokes a defined rc function by name.
func (r *Runner) CallFunc(name string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if r == nil || r.Env == nil {
		return 1
	}
	def, ok := r.Env.GetFunc(name)
	if !ok {
		return 1
	}
	argv := append([]string{name}, args...)
	return r.runFuncCall(def, argv, &ExecPlan{}, r.Env, stdin, stdout, stderr, false)
}

func (r *Runner) runChain(p *ExecPlan, stdin io.Reader, stdout, stderr io.Writer) int {
	status := 0
	for cur := p; cur != nil; cur = cur.Next {
		if r.exitRequested {
			return r.exitCode
		}
		if cur.Background {
			status = r.startBackground(cur, stdin, stdout, stderr)
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
		return r.runPipe(p, p.PipeTo, stdin, stdout, stderr, false)
	}
	return r.runStage(p, stdin, stdout, stderr, false)
}

func (r *Runner) runPipe(left, right *ExecPlan, stdin io.Reader, stdout, stderr io.Writer, background bool) int {
	if background {
		if status, ok := r.runPipeExternal(left, right, stdin, stdout, stderr, true); ok {
			return status
		}
		return r.runPipeFallback(left, right, stdin, stdout, stderr, true)
	}
	if status, ok := r.runPipeExternal(left, right, stdin, stdout, stderr, false); ok {
		return status
	}
	return r.runPipeFallback(left, right, stdin, stdout, stderr, false)
}

func (r *Runner) runPipeFallback(left, right *ExecPlan, stdin io.Reader, stdout, stderr io.Writer, background bool) int {
	pr, pw := io.Pipe()
	leftDone := make(chan int, 1)
	rightDone := make(chan int, 1)

	go func() {
		leftDone <- r.runStage(left, stdin, pw, stderr, background)
		_ = pw.Close()
	}()
	go func() {
		rightDone <- r.runStage(right, pr, stdout, stderr, background)
		_ = pr.Close()
	}()

	_ = <-leftDone
	status := <-rightDone
	return status
}

type stagePrep struct {
	argv []string
	env  *Env
}

func (r *Runner) prepareExternal(p *ExecPlan) (stagePrep, bool, error) {
	if p == nil || p.Kind == PlanFnDef || p.Kind == PlanAssign {
		return stagePrep{}, false, nil
	}
	execEnv := r.Env
	if len(p.Prefix) > 0 {
		child := NewChild(r.Env)
		for _, pref := range p.Prefix {
			vals, err := ExpandValue(pref.Val, child)
			if err != nil {
				return stagePrep{}, false, err
			}
			child.Set(pref.Name, vals)
		}
		execEnv = child
	}
	argv, err := r.expandArgv(p, execEnv)
	if err != nil {
		return stagePrep{}, false, err
	}
	if len(argv) == 0 {
		return stagePrep{}, false, nil
	}
	if _, ok := execEnv.GetFunc(argv[0]); ok {
		return stagePrep{}, false, nil
	}
	if _, ok := r.Builtins[argv[0]]; ok {
		return stagePrep{}, false, nil
	}
	return stagePrep{argv: argv, env: execEnv}, true, nil
}

func (r *Runner) runPipeExternal(left, right *ExecPlan, stdin io.Reader, stdout, stderr io.Writer, background bool) (int, bool) {
	leftPrep, ok, err := r.prepareExternal(left)
	if err != nil {
		return 1, true
	}
	if !ok {
		return 0, false
	}
	rightPrep, ok, err := r.prepareExternal(right)
	if err != nil {
		return 1, true
	}
	if !ok {
		return 0, false
	}
	r.tracef("+ %s\n", strings.Join(leftPrep.argv, " "))
	r.tracef("+ %s\n", strings.Join(rightPrep.argv, " "))

	pr, pw, err := os.Pipe()
	if err != nil {
		return 1, true
	}
	leftCmd, leftCleanup, err := buildCmd(leftPrep.argv, left, stdin, pw, stderr)
	if err != nil {
		_ = pw.Close()
		_ = pr.Close()
		return 1, true
	}
	rightCmd, rightCleanup, err := buildCmd(rightPrep.argv, right, pr, stdout, stderr)
	if err != nil {
		leftCleanup()
		_ = pw.Close()
		_ = pr.Close()
		return 1, true
	}
	defer leftCleanup()
	defer rightCleanup()

	leftCmd.SysProcAttr = &unix.SysProcAttr{Setpgid: true}
	if err := leftCmd.Start(); err != nil {
		_ = pw.Close()
		_ = pr.Close()
		return exitStatus(err), true
	}
	leader := leftCmd.Process.Pid

	rightCmd.SysProcAttr = &unix.SysProcAttr{Setpgid: true, Pgid: leader}
	if err := rightCmd.Start(); err != nil {
		_ = leftCmd.Process.Kill()
		_ = leftCmd.Wait()
		_ = pw.Close()
		_ = pr.Close()
		return exitStatus(err), true
	}
	_ = pw.Close()
	_ = pr.Close()

	if background {
		job := r.onBackgroundStart(leader, []int{leftCmd.Process.Pid, rightCmd.Process.Pid}, strings.Join(leftPrep.argv, " ")+" | "+strings.Join(rightPrep.argv, " "))
		go r.waitJobPids(job, []int{leftCmd.Process.Pid, rightCmd.Process.Pid})
		return 0, true
	}
	r.attachForeground(leader)
	leftDone := make(chan int, 1)
	rightDone := make(chan int, 1)
	go func() {
		err := leftCmd.Wait()
		leftDone <- exitStatus(err)
	}()
	go func() {
		err := rightCmd.Wait()
		rightDone <- exitStatus(err)
	}()
	_ = <-leftDone
	status := <-rightDone
	r.restoreForeground()
	return status, true
}

func (r *Runner) runStage(p *ExecPlan, stdin io.Reader, stdout, stderr io.Writer, background bool) int {
	if p == nil {
		return 0
	}
	switch p.Kind {
	case PlanIf:
		condStatus := r.runAST(p.IfCond, stdin, stdout, stderr)
		if condStatus == 0 {
			return r.runAST(p.IfBody, stdin, stdout, stderr)
		}
		if p.IfElse != nil {
			return r.runAST(p.IfElse, stdin, stdout, stderr)
		}
		return condStatus
	case PlanFor:
		return r.runFor(p, stdin, stdout, stderr)
	case PlanWhile:
		return r.runWhile(p, stdin, stdout, stderr)
	case PlanSwitch:
		return r.runSwitch(p, stdin, stdout, stderr)
	case PlanNot:
		status := r.runAST(p.NotBody, stdin, stdout, stderr)
		if status == 0 {
			return 1
		}
		return 0
	case PlanSubshell:
		child := NewChild(r.Env)
		return r.runASTWithEnv(child, p.SubBody, stdin, stdout, stderr)
	case PlanTwiddle:
		return r.runMatch(p)
	case PlanFnRm:
		if p.Func != nil {
			r.Env.UnsetFunc(p.Func.Name)
		}
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
		return r.runFuncCall(def, argv, p, execEnv, stdin, stdout, stderr, background)
	}
	if builtin, ok := r.Builtins[argv[0]]; ok {
		return r.runBuiltin(builtin, argv, p, execEnv, stdin, stdout, stderr)
	}
	return r.runExternal(argv, p, stdin, stdout, stderr, background, 0)
}

func (r *Runner) runBuiltin(builtin Builtin, argv []string, p *ExecPlan, env *Env, stdin io.Reader, stdout, stderr io.Writer) int {
	in := stdin
	out := stdout
	errOut := stderr
	files, err := applyRedirs(p, &in, &out, &errOut)
	if err != nil {
		return 1
	}
	for _, f := range files {
		defer f.Close()
	}
	orig := r.Env
	r.Env = env
	status := builtin(in, out, errOut, argv, r)
	r.Env = orig
	return status
}

func (r *Runner) runExternal(argv []string, p *ExecPlan, stdin io.Reader, stdout, stderr io.Writer, background bool, wantPgid int) int {
	cmd, cleanup, err := buildCmd(argv, p, stdin, stdout, stderr)
	if err != nil {
		return 127
	}
	if cmd == nil {
		return 0
	}
	defer cleanup()
	if wantPgid != 0 {
		cmd.SysProcAttr = &unix.SysProcAttr{Setpgid: true, Pgid: wantPgid}
	} else {
		cmd.SysProcAttr = &unix.SysProcAttr{Setpgid: true}
	}
	if err := cmd.Start(); err != nil {
		return exitStatus(err)
	}
	if background {
		pgid, err := unix.Getpgid(cmd.Process.Pid)
		if err != nil {
			pgid = cmd.Process.Pid
		}
		job := r.onBackgroundStart(pgid, []int{cmd.Process.Pid}, strings.Join(argv, " "))
		go r.waitJobPids(job, []int{cmd.Process.Pid})
		return 0
	}
	r.attachForeground(cmd.Process.Pid)
	err = cmd.Wait()
	r.restoreForeground()
	return exitStatus(err)
}

func (r *Runner) runFuncCall(def FuncDef, argv []string, p *ExecPlan, env *Env, stdin io.Reader, stdout, stderr io.Writer, background bool) int {
	in := stdin
	out := stdout
	errOut := stderr
	files, err := applyRedirs(p, &in, &out, &errOut)
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
	if background {
		go r.runChain(bodyPlan, in, out, stderr)
		r.Env = origEnv
		return 0
	}
	status := r.runChain(bodyPlan, in, out, errOut)
	r.Env = origEnv
	return status
}

func (r *Runner) runAST(n *parse.Node, stdin io.Reader, stdout, stderr io.Writer) int {
	if n == nil {
		return 0
	}
	plan, err := BuildPlan(n, r.Env)
	if err != nil {
		return 1
	}
	return r.runChain(plan, stdin, stdout, stderr)
}

func (r *Runner) runASTWithEnv(env *Env, n *parse.Node, stdin io.Reader, stdout, stderr io.Writer) int {
	if n == nil {
		return 0
	}
	plan, err := BuildPlan(n, env)
	if err != nil {
		return 1
	}
	orig := r.Env
	r.Env = env
	status := r.runChain(plan, stdin, stdout, stderr)
	r.Env = orig
	return status
}

func (r *Runner) runFor(p *ExecPlan, stdin io.Reader, stdout, stderr io.Writer) int {
	if p.ForName == "" {
		return 1
	}
	var list []string
	if p.ForList != nil {
		vals, err := ExpandValue(p.ForList, r.Env)
		if err != nil {
			return 1
		}
		list = vals
	} else {
		list = r.Env.Get("*")
	}
	if len(list) == 0 {
		return 0
	}
	status := 0
	for _, val := range list {
		r.Env.Set(p.ForName, []string{val})
		status = r.runAST(p.ForBody, stdin, stdout, stderr)
		r.Env.SetStatus(status)
		if r.exitRequested {
			break
		}
	}
	return status
}

func (r *Runner) runWhile(p *ExecPlan, stdin io.Reader, stdout, stderr io.Writer) int {
	status := 0
	for {
		cond := r.runAST(p.WhileCond, stdin, stdout, stderr)
		if cond != 0 {
			return status
		}
		status = r.runAST(p.WhileBody, stdin, stdout, stderr)
		r.Env.SetStatus(status)
		if r.exitRequested {
			return status
		}
	}
}

func (r *Runner) runSwitch(p *ExecPlan, stdin io.Reader, stdout, stderr io.Writer) int {
	arg := ""
	if p.SwitchArg != nil {
		vals, err := ExpandWordNoGlob(p.SwitchArg, r.Env)
		if err != nil {
			return 1
		}
		if len(vals) > 0 {
			arg = vals[0]
		}
	}
	cases, err := switchCases(p.SwitchBody, r.Env)
	if err != nil {
		return 1
	}
	if len(cases) == 0 {
		return 0
	}
	status := 0
	matched := false
	for _, c := range cases {
		if !matched && matchAnyPattern(arg, c.Patterns) {
			matched = true
		}
		if matched {
			status = r.runAST(c.Body, stdin, stdout, stderr)
			r.Env.SetStatus(status)
		}
	}
	return status
}

func (r *Runner) runMatch(p *ExecPlan) int {
	subjects, err := ExpandWordNoGlob(p.MatchSubj, r.Env)
	if err != nil {
		return 1
	}
	if len(subjects) == 0 {
		return 1
	}
	patterns, err := ExpandWordsNoGlob(p.MatchPats, r.Env)
	if err != nil {
		return 1
	}
	if len(patterns) == 0 {
		return 1
	}
	for _, subj := range subjects {
		for _, pat := range patterns {
			if rcMatch(pat, subj) {
				return 0
			}
		}
	}
	return 1
}

func buildCmd(argv []string, p *ExecPlan, stdin io.Reader, stdout, stderr io.Writer) (*exec.Cmd, func(), error) {
	if p == nil || len(argv) == 0 {
		return nil, func() {}, nil
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	files, err := applyRedirs(p, &cmd.Stdin, &cmd.Stdout, &cmd.Stderr)
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

func (r *Runner) startBackground(p *ExecPlan, stdin io.Reader, stdout, stderr io.Writer) int {
	if p == nil {
		return 0
	}
	if p.PipeTo != nil {
		go r.runPipe(p, p.PipeTo, stdin, stdout, stderr, true)
		return 0
	}
	return r.runStage(p, stdin, stdout, stderr, true)
}

func (r *Runner) onBackgroundStart(pgid int, pids []int, cmd string) *Job {
	for _, pid := range pids {
		r.addAPID(pid)
	}
	return r.addJob(pgid, pids, cmd)
}

func (r *Runner) waitJobPids(job *Job, pids []int) {
	if job == nil {
		return
	}
	exit := 0
	for _, pid := range pids {
		for {
			var ws unix.WaitStatus
			_, err := unix.Wait4(pid, &ws, 0, nil)
			if err == unix.EINTR {
				continue
			}
			if err != nil {
				break
			}
			exit = ws.ExitStatus()
			break
		}
		r.removeAPID(pid)
	}
	r.markJobDone(job.Pgid, exit)
	select {
	case job.Done <- exit:
	default:
	}
}

func (r *Runner) attachForeground(pid int) {
	if !r.Interactive || r.TTYFD <= 0 {
		return
	}
	pgid, err := unix.Getpgid(pid)
	if err != nil {
		return
	}
	r.attachForegroundPgid(pgid)
}

func (r *Runner) restoreForeground() {
	if !r.Interactive || r.TTYFD <= 0 {
		r.mu.Lock()
		r.ForegroundPgid = 0
		r.mu.Unlock()
		return
	}
	if r.ShellPgid != 0 {
		signal.Ignore(syscall.SIGTTOU)
		defer signal.Reset(syscall.SIGTTOU)
		err := r.setForegroundPgrp(r.ShellPgid)
		if err != nil {
			r.tracef("tcsetpgrp restore failed: %v\n", err)
		}
	}
	r.mu.Lock()
	r.ForegroundPgid = 0
	r.mu.Unlock()
}

func (r *Runner) attachForegroundPgid(pgid int) {
	if !r.Interactive || r.TTYFD <= 0 {
		return
	}
	signal.Ignore(syscall.SIGTTOU)
	defer signal.Reset(syscall.SIGTTOU)
	err := r.setForegroundPgrp(pgid)
	if err != nil {
		r.tracef("tcsetpgrp failed: %v\n", err)
		return
	}
	r.mu.Lock()
	r.ForegroundPgid = pgid
	r.mu.Unlock()
}

func (r *Runner) setForegroundPgrp(pgid int) error {
	if r.TTYFD <= 0 {
		return fmt.Errorf("invalid tty fd")
	}
	return unix.IoctlSetPointerInt(r.TTYFD, unix.TIOCSPGRP, pgid)
}

func (r *Runner) tracef(format string, args ...any) {
	if !r.Trace || r.TraceWriter == nil {
		return
	}
	fmt.Fprintf(r.TraceWriter, format, args...)
}

func applyRedirs(p *ExecPlan, stdin *io.Reader, stdout, stderr *io.Writer) ([]*os.File, error) {
	if p == nil {
		return nil, nil
	}
	var files []*os.File
	for _, r := range p.Redirs {
		if r.Op == "dup" {
			if err := applyDup(r, stdin, stdout, stderr, &files); err != nil {
				return files, err
			}
			continue
		}
		if len(r.Target) == 0 {
			continue
		}
		path := r.Target[0]
		fd := r.Fd
		if fd < 0 {
			fd = defaultRedirFD(r.Op)
		}
		switch r.Op {
		case ">":
			f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o666)
			if err != nil {
				return files, err
			}
			if err := assignFD(fd, stdin, stdout, stderr, f); err != nil {
				_ = f.Close()
				return files, err
			}
			files = append(files, f)
		case ">>":
			f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o666)
			if err != nil {
				return files, err
			}
			if err := assignFD(fd, stdin, stdout, stderr, f); err != nil {
				_ = f.Close()
				return files, err
			}
			files = append(files, f)
		case "<":
			f, err := os.Open(path)
			if err != nil {
				return files, err
			}
			if err := assignFD(fd, stdin, stdout, stderr, f); err != nil {
				_ = f.Close()
				return files, err
			}
			files = append(files, f)
		case "<>":
			f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o666)
			if err != nil {
				return files, err
			}
			if err := assignFD(fd, stdin, stdout, stderr, f); err != nil {
				_ = f.Close()
				return files, err
			}
			files = append(files, f)
		case "<<", "<<<":
			return files, fmt.Errorf("heredoc not implemented")
		default:
			return files, fmt.Errorf("unsupported redirection: %s", r.Op)
		}
	}
	return files, nil
}

func defaultRedirFD(op string) int {
	if strings.HasPrefix(op, "<") {
		return 0
	}
	return 1
}

func assignFD(fd int, stdin *io.Reader, stdout, stderr *io.Writer, f *os.File) error {
	switch fd {
	case 0:
		*stdin = f
	case 1:
		*stdout = f
	case 2:
		*stderr = f
	default:
		return fmt.Errorf("unsupported fd %d", fd)
	}
	return nil
}

func applyDup(r RedirPlan, stdin *io.Reader, stdout, stderr *io.Writer, files *[]*os.File) error {
	if r.Fd < 0 {
		return fmt.Errorf("dup missing target fd")
	}
	if r.Close {
		return closeFD(r.Fd, stdin, stdout, stderr, files)
	}
	if r.Fd == r.DupTo {
		return nil
	}
	srcWriter, srcWriterOK := writerForFD(r.DupTo, stdin, stdout, stderr)
	srcReader, srcReaderOK := readerForFD(r.DupTo, stdin, stdout, stderr)
	switch r.Fd {
	case 0:
		if !srcReaderOK {
			return fmt.Errorf("dup source fd %d is not readable", r.DupTo)
		}
		*stdin = srcReader
	case 1:
		if !srcWriterOK {
			return fmt.Errorf("dup source fd %d is not writable", r.DupTo)
		}
		*stdout = srcWriter
	case 2:
		if !srcWriterOK {
			return fmt.Errorf("dup source fd %d is not writable", r.DupTo)
		}
		*stderr = srcWriter
	default:
		return fmt.Errorf("unsupported fd %d", r.Fd)
	}
	return nil
}

func closeFD(fd int, stdin *io.Reader, stdout, stderr *io.Writer, files *[]*os.File) error {
	var f *os.File
	var err error
	switch fd {
	case 0:
		f, err = os.Open(os.DevNull)
	case 1, 2:
		f, err = os.OpenFile(os.DevNull, os.O_WRONLY, 0o666)
	default:
		return fmt.Errorf("unsupported fd %d", fd)
	}
	if err != nil {
		return err
	}
	*files = append(*files, f)
	return assignFD(fd, stdin, stdout, stderr, f)
}

func writerForFD(fd int, stdin *io.Reader, stdout, stderr *io.Writer) (io.Writer, bool) {
	switch fd {
	case 1:
		return *stdout, true
	case 2:
		return *stderr, true
	case 0:
		if w, ok := (*stdin).(io.Writer); ok {
			return w, true
		}
	}
	return nil, false
}

func readerForFD(fd int, stdin *io.Reader, stdout, stderr *io.Writer) (io.Reader, bool) {
	switch fd {
	case 0:
		return *stdin, true
	case 1:
		if r, ok := (*stdout).(io.Reader); ok {
			return r, true
		}
	case 2:
		if r, ok := (*stderr).(io.Reader); ok {
			return r, true
		}
	}
	return nil, false
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
