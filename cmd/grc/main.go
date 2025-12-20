package main

import (
	"flag"
	"fmt"
	"os"

	"grc/internal/eval"
	"grc/internal/parse"
)

func main() {
	noexec := flag.Bool("n", false, "parse and build only")
	printplan := flag.Bool("p", false, "print plan")
	trace := flag.Bool("x", false, "trace commands")
	flag.Parse()

	ast, err := parse.ParseAll(os.Stdin)
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
	if *printplan {
		fmt.Fprint(os.Stderr, eval.DumpPlan(plan))
	}
	if *noexec {
		os.Exit(0)
	}
	runner := &eval.Runner{Env: env, Trace: *trace, TraceWriter: os.Stderr}
	result := runner.RunPlan(plan, os.Stdin, os.Stdout, os.Stderr)
	if runner.ExitRequested() {
		os.Exit(runner.ExitCode())
	}
	if result.Status != 0 {
		os.Exit(result.Status)
	}
}
