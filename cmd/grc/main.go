package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterh/liner"
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
			_, _ = line.ReadHistory(f)
			_ = f.Close()
		}
	}

	env := eval.NewEnv(nil)
	for {
		input, err := line.Prompt("grc> ")
		if err == liner.ErrPromptAborted {
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
		runner := &eval.Runner{Env: env, Trace: trace, TraceWriter: os.Stderr}
		result := runner.RunPlan(plan, strings.NewReader(""), os.Stdout, os.Stderr)
		if runner.ExitRequested() {
			os.Exit(runner.ExitCode())
		}
		if result.Status != 0 {
			fmt.Fprintln(os.Stderr, result.Status)
		}
	}

	if historyPath != "" {
		if f, err := os.Create(historyPath); err == nil {
			_, _ = line.WriteHistory(f)
			_ = f.Close()
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
