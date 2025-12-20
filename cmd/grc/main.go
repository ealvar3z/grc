package main

import (
	"flag"
	"fmt"
	"os"

	"grc/internal/eval"
	"grc/internal/parse"
)

func main() {
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
	runner := &eval.Runner{Env: env, Trace: *trace, TraceWriter: os.Stderr}
	result := runner.RunPlan(plan, os.Stdin, os.Stdout, os.Stderr)
	if runner.ExitRequested() {
		os.Exit(runner.ExitCode())
	}
	if result.Status != 0 {
		os.Exit(result.Status)
	}
}
