package eval

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"grc/internal/parse"
)

func TestFnDefineAndCall(t *testing.T) {
	if !haveCmd(t, "printf") {
		t.Skip("printf not available")
	}
	env := NewEnv(nil)
	input := "fn f { printf hi }; f\n"
	ast, err := parse.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	plan, err := BuildPlan(ast, env)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	var out bytes.Buffer
	res := (&Runner{Env: env}).RunPlan(plan, strings.NewReader(""), &out, io.Discard)
	if res.Status != 0 {
		t.Fatalf("expected status 0, got %d", res.Status)
	}
	if out.String() != "hi" {
		t.Fatalf("unexpected stdout: %q", out.String())
	}
	got := env.Get("status")
	if len(got) != 1 || got[0] != "0" {
		t.Fatalf("unexpected status: %v", got)
	}
}

func TestFnArgsAndConcat(t *testing.T) {
	if !haveCmd(t, "printf") {
		t.Skip("printf not available")
	}
	env := NewEnv(nil)
	input := "fn f { printf %s $1^$2 }; f a b\n"
	ast, err := parse.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	plan, err := BuildPlan(ast, env)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	var out bytes.Buffer
	res := (&Runner{Env: env}).RunPlan(plan, strings.NewReader(""), &out, io.Discard)
	if res.Status != 0 {
		t.Fatalf("expected status 0, got %d", res.Status)
	}
	if out.String() != "ab" {
		t.Fatalf("unexpected stdout: %q", out.String())
	}
}

func TestFnStarArgs(t *testing.T) {
	if !haveCmd(t, "printf") {
		t.Skip("printf not available")
	}
	env := NewEnv(nil)
	input := "fn f { printf %s $* }; f a b\n"
	ast, err := parse.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	plan, err := BuildPlan(ast, env)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	var out bytes.Buffer
	res := (&Runner{Env: env}).RunPlan(plan, strings.NewReader(""), &out, io.Discard)
	if res.Status != 0 {
		t.Fatalf("expected status 0, got %d", res.Status)
	}
	if out.String() != "ab" {
		t.Fatalf("unexpected stdout: %q", out.String())
	}
}

func TestFnDynamicScope(t *testing.T) {
	if !haveCmd(t, "printf") {
		t.Skip("printf not available")
	}
	env := NewEnv(nil)
	env.Set1("x", "world")
	input := "fn f { printf %s $x }; f\n"
	ast, err := parse.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	plan, err := BuildPlan(ast, env)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	var out bytes.Buffer
	res := (&Runner{Env: env}).RunPlan(plan, strings.NewReader(""), &out, io.Discard)
	if res.Status != 0 {
		t.Fatalf("expected status 0, got %d", res.Status)
	}
	if out.String() != "world" {
		t.Fatalf("unexpected stdout: %q", out.String())
	}
}
