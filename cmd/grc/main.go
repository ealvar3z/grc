package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/peterh/liner"
	"golang.org/x/sys/unix"
	"golang.org/x/term"

	"grc/internal/eval"
	"grc/internal/parse"
)

func main() {
	noexec := flag.Bool("n", false, "parse and build only")
	printplan := flag.Bool("p", false, "print plan")
	trace := flag.Bool("x", false, "trace commands")
	flag.Parse()

	if term.IsTerminal(int(os.Stdin.Fd())) {
		runInteractive(*noexec, *printplan, *trace)
		return
	}
	runScript(*noexec, *printplan, *trace, os.Stdin)
}

func runScript(noexec, printplan, trace bool, rd io.Reader) {
	ast, err := parse.ParseAll(rd)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	env := eval.NewEnv(nil)
	plan, err := eval.BuildPlan(ast, env)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if printplan {
		fmt.Fprint(os.Stderr, eval.DumpPlan(plan))
	}
	if noexec {
		os.Exit(0)
	}
	runner := &eval.Runner{Env: env, Trace: trace, TraceWriter: os.Stderr}
	result := runner.RunPlan(plan, os.Stdin, os.Stdout, os.Stderr)
	if runner.ExitRequested() {
		os.Exit(runner.ExitCode())
	}
	if result.Status != 0 {
		os.Exit(result.Status)
	}
}

func runInteractive(noexec, printplan, trace bool) {
	line := liner.NewLiner()
	defer line.Close()
	line.SetCtrlCAborts(true)

	historyPath := historyFile()
	if historyPath != "" {
		if f, err := os.Open(historyPath); err == nil {
			defer f.Close()
			_, _ = line.ReadHistory(f)
		}
	}

	env := eval.NewEnv(nil)
	runner := &eval.Runner{
		Env:         env,
		Trace:       trace,
		TraceWriter: os.Stderr,
	}
	ttyfd := int(os.Stdin.Fd())
	runner.Interactive = true
	runner.TTYFD = ttyfd
	if ttyfd > 0 {
		pid := os.Getpid()
		_ = syscall.Setpgid(0, 0)
		shellPgid, _ := syscall.Getpgid(pid)
		runner.ShellPgid = shellPgid
		signal.Ignore(syscall.SIGTTOU)
		_ = unix.IoctlSetPointerInt(ttyfd, unix.TIOCSPGRP, shellPgid)
		signal.Reset(syscall.SIGTTOU)
	}
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt)
	defer signal.Stop(sigc)
	go func() {
		for range sigc {
			pgid := runner.Foreground()
			if pgid != 0 {
				_ = unix.Kill(-pgid, unix.SIGINT)
			} else {
				fmt.Fprintln(os.Stderr)
			}
		}
	}()
	for {
		prompt := "; "
		if vals := env.Get("prompt"); len(vals) > 0 {
			prompt = strings.Join(vals, " ") + " "
		}
		input, err := line.Prompt(prompt)
		if err == liner.ErrPromptAborted {
			fmt.Fprintln(os.Stderr)
			continue
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			break
		}
		if strings.TrimSpace(input) == "" {
			continue
		}
		line.AppendHistory(input)

		ast, err := parse.ParseAll(strings.NewReader(input + "\n"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		plan, err := eval.BuildPlan(ast, env)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		if printplan {
			fmt.Fprint(os.Stderr, eval.DumpPlan(plan))
		}
		if noexec {
			continue
		}
		result := runner.RunPlan(plan, os.Stdin, os.Stdout, os.Stderr)
		if runner.ExitRequested() {
			os.Exit(runner.ExitCode())
		}
		_ = result
	}

	if historyPath != "" {
		if f, err := os.Create(historyPath); err == nil {
			defer f.Close()
			_, _ = line.WriteHistory(f)
		}
	}
}

func historyFile() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".grc_history")
}
