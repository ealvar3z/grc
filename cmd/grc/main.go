package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
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
	opts, args := parseArgs(os.Args[1:])
	env := eval.NewEnv(nil)
	initEnv(env)
	initStar(env, os.Args[0], args)

	if opts.command != "" {
		runCommand(opts, env, opts.command)
		return
	}
	if len(args) > 0 && !opts.readStdin {
		runDotFile(opts, env, args)
		return
	}
	interactive := opts.interactive
	if !opts.interactiveForced && !opts.interactiveDisabled {
		interactive = term.IsTerminal(int(os.Stdin.Fd()))
	}
	if interactive {
		runInteractive(opts, env)
		return
	}
	runScript(opts, env, os.Stdin)
}

func runScript(opts options, env *eval.Env, rd io.Reader) {
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
	if opts.printplan {
		fmt.Fprint(os.Stderr, eval.DumpPlan(plan))
	}
	if opts.noexec {
		os.Exit(0)
	}
	runner := &eval.Runner{Env: env, Trace: opts.trace, TraceWriter: os.Stderr}
	if self, err := os.Executable(); err == nil {
		runner.SelfPath = self
	}
	result := runner.RunPlan(plan, os.Stdin, os.Stdout, os.Stderr)
	if runner.ExitRequested() {
		os.Exit(runner.ExitCode())
	}
	if result.Status != 0 {
		os.Exit(result.Status)
	}
}

func runCommand(opts options, env *eval.Env, cmd string) {
	runScript(opts, env, strings.NewReader(cmd))
}

func runDotFile(opts options, env *eval.Env, args []string) {
	if opts.noexec {
		if len(args) == 0 {
			return
		}
		f, err := os.Open(args[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer f.Close()
		_, err = parse.ParseAll(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	runner := &eval.Runner{Env: env, Trace: opts.trace, TraceWriter: os.Stderr}
	if self, err := os.Executable(); err == nil {
		runner.SelfPath = self
	}
	status := runner.RunPlan(
		&eval.ExecPlan{Kind: eval.PlanCmd, Argv: append([]string{"."}, args...)},
		os.Stdin,
		os.Stdout,
		os.Stderr,
	)
	if runner.ExitRequested() {
		os.Exit(runner.ExitCode())
	}
	if status.Status != 0 {
		os.Exit(status.Status)
	}
}

func runInteractive(opts options, env *eval.Env) {
	line := liner.NewLiner()
	line.SetCtrlCAborts(true)
	defer func() {
		if line != nil {
			line.Close()
		}
	}()

	runner := &eval.Runner{
		Env:         env,
		Trace:       opts.trace,
		TraceWriter: os.Stderr,
	}
	if self, err := os.Executable(); err == nil {
		runner.SelfPath = self
	}
	line.SetCompleter(func(input string) []string {
		return completeLine(input, env, runner)
	})
	runner.JobControl = false
	historyPath := historyPathFromEnv(env)
	var historyLines []string
	if historyPath != "" {
		if f, err := os.Open(historyPath); err == nil {
			defer f.Close()
			_, _ = line.ReadHistory(f)
		}
	}
	ttyfd := int(os.Stdin.Fd())
	runner.Interactive = true
	runner.TTYFD = ttyfd
	var origState *term.State
	if ttyfd > 0 {
		if st, err := term.GetState(ttyfd); err == nil {
			origState = st
		}
	}
	initJobControl(runner)
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
		if !opts.noexec {
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
		historyLines = append(historyLines, input)
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
		if opts.printplan {
			fmt.Fprint(os.Stderr, eval.DumpPlan(plan))
		}
		if opts.noexec {
			continue
		}
		line.Close()
		if origState != nil {
			_ = term.Restore(ttyfd, origState)
		}
		result := runner.RunPlan(plan, os.Stdin, os.Stdout, os.Stderr)
		line = liner.NewLiner()
		line.SetCtrlCAborts(true)
		line.SetCompleter(func(input string) []string {
			return completeLine(input, env, runner)
		})
		if historyPath != "" {
			if f, err := os.Open(historyPath); err == nil {
				_, _ = line.ReadHistory(f)
				_ = f.Close()
			}
		}
		for _, h := range historyLines {
			line.AppendHistory(h)
		}
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

type options struct {
	noexec              bool
	printplan           bool
	trace               bool
	readStdin           bool
	interactive         bool
	interactiveForced   bool
	interactiveDisabled bool
	command             string
}

func parseArgs(args []string) (options, []string) {
	var opts options
	var rest []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			rest = append(rest, args[i:]...)
			break
		}
		if arg == "--" {
			rest = append(rest, args[i+1:]...)
			break
		}
		for j := 1; j < len(arg); j++ {
			switch arg[j] {
			case 'c':
				if i+1 < len(args) {
					opts.command = args[i+1]
					i++
				}
			case 'n':
				opts.noexec = true
			case 'p':
				opts.printplan = true
			case 'x':
				opts.trace = true
			case 's':
				opts.readStdin = true
			case 'i':
				opts.interactive = true
				opts.interactiveForced = true
			case 'I':
				opts.interactive = false
				opts.interactiveDisabled = true
			case 'l':
				// login flag; no behavior yet
			case 'd', 'e', 'o', 'v':
				// reserved: parser/exec flags in rc
			default:
				// unknown flag ignored
			}
		}
	}
	return opts, rest
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

func completeLine(line string, env *eval.Env, runner *eval.Runner) []string {
	if line == "" {
		return nil
	}
	if strings.Count(line, "'")%2 == 1 {
		return nil
	}
	token, start := lastToken(line)
	if token == "" {
		return nil
	}
	if strings.HasPrefix(token, "$") {
		return completeVars(token, env)
	}
	if strings.Contains(token, "/") || strings.HasPrefix(token, ".") {
		return completePath(token)
	}
	if isCommandPosition(line, start) {
		return completeCommand(token, env, runner)
	}
	return completePath(token)
}

func lastToken(line string) (string, int) {
	i := len(line)
	for i > 0 {
		switch line[i-1] {
		case ' ', '\t', '\n':
			return line[i:], i
		}
		i--
	}
	return line, 0
}

func isCommandPosition(line string, start int) bool {
	i := start - 1
	for i >= 0 {
		switch line[i] {
		case ' ', '\t', '\n':
			i--
			continue
		case ';', '|', '&', '(', ')', '{', '}':
			return true
		default:
			return false
		}
	}
	return true
}

func completeVars(prefix string, env *eval.Env) []string {
	if env == nil {
		return nil
	}
	raw := strings.TrimPrefix(prefix, "$")
	var out []string
	for name := range env.Snapshot() {
		if !strings.HasPrefix(name, raw) {
			continue
		}
		out = append(out, "$"+name)
	}
	sort.Strings(out)
	return out
}

func completeCommand(prefix string, env *eval.Env, runner *eval.Runner) []string {
	seen := make(map[string]struct{})
	var out []string
	var builtins []string
	if runner != nil && runner.Builtins != nil {
		for name := range runner.Builtins {
			builtins = append(builtins, name)
		}
	} else {
		builtins = eval.BuiltinNames()
	}
	for _, name := range builtins {
		if strings.HasPrefix(name, prefix) {
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	if env != nil {
		for _, name := range env.FuncNames() {
			if !strings.HasPrefix(name, prefix) {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	for _, name := range completePathCommands(prefix, env) {
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func completePathCommands(prefix string, env *eval.Env) []string {
	dirs := pathListFromEnv(env)
	seen := make(map[string]struct{})
	var out []string
	for _, dir := range dirs {
		if dir == "" {
			dir = "."
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasPrefix(name, prefix) {
				continue
			}
			if strings.HasPrefix(name, ".") && !strings.HasPrefix(prefix, ".") {
				continue
			}
			full := filepath.Join(dir, name)
			if !isExecutable(full, entry) {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func completePath(prefix string) []string {
	dir, base := filepath.Split(prefix)
	searchDir := dir
	if searchDir == "" {
		searchDir = "."
	}
	entries, err := os.ReadDir(searchDir)
	if err != nil {
		return nil
	}
	var out []string
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, base) {
			continue
		}
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(base, ".") {
			continue
		}
		candidate := name
		if dir != "" {
			candidate = dir + name
		}
		if entry.IsDir() {
			candidate += string(os.PathSeparator)
		}
		out = append(out, candidate)
	}
	sort.Strings(out)
	return out
}

func isExecutable(path string, entry os.DirEntry) bool {
	info, err := entry.Info()
	if err != nil {
		return false
	}
	if !info.Mode().IsRegular() {
		return false
	}
	if info.Mode().Perm()&0o111 == 0 {
		return false
	}
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return true
}

func pathListFromEnv(env *eval.Env) []string {
	if env != nil {
		if vals := env.Get("path"); len(vals) > 0 {
			return vals
		}
	}
	if p := os.Getenv("PATH"); p != "" {
		return strings.Split(p, string(os.PathListSeparator))
	}
	return []string{""}
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

func initStar(env *eval.Env, argv0 string, args []string) {
	if env == nil {
		return
	}
	env.Set("0", []string{argv0})
	env.Set("*", args)
}

func initJobControl(runner *eval.Runner) {
	ttyfd := runner.TTYFD
	if ttyfd <= 0 {
		return
	}
	pid := os.Getpid()
	_ = syscall.Setpgid(0, 0)
	shellPgid, _ := syscall.Getpgid(pid)
	runner.ShellPgid = shellPgid
	runner.Interactive = true

	for {
		pgrp, err := unix.IoctlGetInt(ttyfd, unix.TIOCGPGRP)
		if err != nil || pgrp == shellPgid {
			break
		}
		_ = unix.Kill(-shellPgid, unix.SIGTTIN)
	}
	signal.Ignore(syscall.SIGTTOU)
	defer signal.Reset(syscall.SIGTTOU)
	_ = unix.IoctlSetPointerInt(ttyfd, unix.TIOCSPGRP, shellPgid)
}
