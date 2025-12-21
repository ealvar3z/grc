package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/peterh/liner"
	"golang.org/x/sys/unix"
	"golang.org/x/term"

	"grc/internal/eval"
	"grc/internal/parse"
)

const version = "dev"

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
	env := eval.NewEnv(nil)
	initEnv(env)
	ast, err := parse.ParseAll(rd)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
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

	env := eval.NewEnv(nil)
	initEnv(env)
	runner := &eval.Runner{
		Env:         env,
		Trace:       trace,
		TraceWriter: os.Stderr,
	}
	historyPath := historyPathFromEnv(env)
	if historyPath != "" {
		if f, err := os.Open(historyPath); err == nil {
			defer f.Close()
			_, _ = line.ReadHistory(f)
		}
	}
	ttyfd := int(os.Stdin.Fd())
	runner.Interactive = true
	runner.TTYFD = ttyfd
	if ttyfd > 0 {
		pid := os.Getpid()
		_ = syscall.Setpgid(0, 0)
		shellPgid, _ := syscall.Getpgid(pid)
		runner.ShellPgid = shellPgid
		signal.Ignore(syscall.SIGTTOU, syscall.SIGTTIN, syscall.SIGTSTP)
		defer signal.Reset(syscall.SIGTTOU, syscall.SIGTTIN, syscall.SIGTSTP)
		_ = unix.IoctlSetPointerInt(ttyfd, unix.TIOCSPGRP, shellPgid)
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
		if !noexec {
			if _, ok := env.GetFunc("prompt"); ok {
				_ = runner.CallFunc("prompt", nil, os.Stdin, os.Stdout, os.Stderr)
			}
		}
		prompt1, prompt2 := promptsFromEnv(env)
		input, err := readCommand(line, prompt1, prompt2)
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
		appendHistory(env, input)

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

func historyPathFromEnv(env *eval.Env) string {
	if env == nil {
		return ""
	}
	vals := env.Get("history")
	if len(vals) == 0 || vals[0] == "" {
		return ""
	}
	return vals[0]
}

func appendHistory(env *eval.Env, input string) {
	path := historyPathFromEnv(env)
	if path == "" {
		return
	}
	if shouldSkipHistory(input) {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o666)
	if err != nil {
		return
	}
	defer f.Close()
	if !strings.HasSuffix(input, "\n") {
		input += "\n"
	}
	_, _ = f.WriteString(input)
}

func shouldSkipHistory(input string) bool {
	for _, r := range input {
		switch r {
		case ' ', '\t':
			continue
		case '#', '\n':
			return true
		default:
			return false
		}
	}
	return true
}

func promptsFromEnv(env *eval.Env) (string, string) {
	prompt1 := "; "
	prompt2 := ""
	if env == nil {
		return prompt1, prompt2
	}
	vals := env.Get("prompt")
	if len(vals) == 0 {
		return prompt1, prompt2
	}
	prompt1 = vals[0]
	if len(vals) > 1 {
		prompt2 = vals[1]
	}
	return prompt1, prompt2
}

func readCommand(line *liner.State, prompt1, prompt2 string) (string, error) {
	var buf strings.Builder
	prompt := prompt1
	for {
		input, err := line.Prompt(prompt)
		if err != nil {
			return "", err
		}
		if buf.Len() > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(input)
		if !needsMoreInput(buf.String()) {
			return buf.String(), nil
		}
		prompt = prompt2
	}
}

func needsMoreInput(s string) bool {
	inQuote := false
	brace := 0
	paren := 0
	escaped := false
	for _, r := range s {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && !inQuote {
			escaped = true
			continue
		}
		if r == '\'' {
			inQuote = !inQuote
			continue
		}
		if inQuote {
			continue
		}
		switch r {
		case '{':
			brace++
		case '}':
			if brace > 0 {
				brace--
			}
		case '(':
			paren++
		case ')':
			if paren > 0 {
				paren--
			}
		}
	}
	if escaped {
		return true
	}
	return brace > 0 || paren > 0 || inQuote
}

func initEnv(env *eval.Env) {
	if env == nil {
		return
	}
	env.Set("ifs", []string{" ", "\t", "\n"})
	env.Set("nl", []string{"\n"})
	env.Set("prompt", []string{"; ", ""})
	env.Set("tab", []string{"\t"})
	env.Set("pid", []string{fmt.Sprintf("%d", os.Getpid())})
	env.Set("version", []string{version})
	for _, kv := range os.Environ() {
		if kv == "" {
			continue
		}
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		val := parts[1]
		if key == "PATH" {
			if val == "" {
				env.Set("path", nil)
			} else {
				env.Set("path", strings.Split(val, string(os.PathListSeparator)))
			}
			continue
		}
		env.Set(key, []string{val})
	}
	if vals := env.Get("home"); len(vals) == 0 || vals[0] == "" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			env.Set("home", []string{home})
		}
	}
	if vals := env.Get("path"); len(vals) == 0 {
		if path := os.Getenv("PATH"); path != "" {
			env.Set("path", strings.Split(path, string(os.PathListSeparator)))
		}
	}
	if vals := env.Get("prompt"); len(vals) == 0 {
		env.Set("prompt", []string{"; ", ""})
	}
	if vals := env.Get("ifs"); len(vals) == 0 {
		env.Set("ifs", []string{" ", "\t", "\n"})
	}
	if vals := env.Get("nl"); len(vals) == 0 {
		env.Set("nl", []string{"\n"})
	}
	if vals := env.Get("tab"); len(vals) == 0 {
		env.Set("tab", []string{"\t"})
	}
	if vals := env.Get("pid"); len(vals) == 0 {
		env.Set("pid", []string{fmt.Sprintf("%d", os.Getpid())})
	}
	if vals := env.Get("version"); len(vals) == 0 {
		env.Set("version", []string{version})
	}
}
