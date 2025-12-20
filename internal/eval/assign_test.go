package eval

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"grc/internal/parse"
)

func TestAssignAndEcho(t *testing.T) {
	if !haveCmd(t, "printf") {
		t.Skip("printf not available")
	}
	env := NewEnv(nil)
	input := "x=world\nprintf %s $x\n"
	ast, err := parse.ParseAll(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseAll returned error: %v", err)
	}
	plan, err := BuildPlan(ast, env)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	if plan == nil || plan.Next == nil {
		t.Fatalf("expected assignment followed by command")
	}
	if len(plan.Next.Argv) < 2 {
		t.Fatalf("unexpected argv: %v", plan.Next.Argv)
	}
	var out bytes.Buffer
	res := (&Runner{Env: env}).RunPlan(plan, strings.NewReader(""), &out, io.Discard)
	if res.Status != 0 {
		t.Fatalf("expected status 0, got %d", res.Status)
	}
	vals := env.Get("x")
	if len(vals) != 1 || vals[0] != "world" {
		t.Fatalf("unexpected x: %v", vals)
	}
	if out.String() != "world" {
		t.Fatalf("unexpected stdout: %q", out.String())
	}
	got := env.Get("status")
	if len(got) != 1 || got[0] != "0" {
		t.Fatalf("unexpected status: %v", got)
	}
}

func TestAssignList(t *testing.T) {
	if !haveCmd(t, "printf") {
		t.Skip("printf not available")
	}
	env := NewEnv(nil)
	input := "x=(a b)\nprintf %s $x\n"
	ast, err := parse.ParseAll(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseAll returned error: %v", err)
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
	vals := env.Get("x")
	if len(vals) != 2 || vals[0] != "a" || vals[1] != "b" {
		t.Fatalf("unexpected x: %v", vals)
	}
	if out.String() != "ab" {
		t.Fatalf("unexpected stdout: %q", out.String())
	}
	got := env.Get("status")
	if len(got) != 1 || got[0] != "0" {
		t.Fatalf("unexpected status: %v", got)
	}
}

func TestAssignEmpty(t *testing.T) {
	if !haveCmd(t, "printf") {
		t.Skip("printf not available")
	}
	env := NewEnv(nil)
	input := "x=()\nprintf %s $x\n"
	ast, err := parse.ParseAll(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseAll returned error: %v", err)
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
	vals := env.Get("x")
	if len(vals) != 0 {
		t.Fatalf("unexpected x: %v", vals)
	}
	if out.String() != "" {
		t.Fatalf("unexpected stdout: %q", out.String())
	}
	got := env.Get("status")
	if len(got) != 1 || got[0] != "0" {
		t.Fatalf("unexpected status: %v", got)
	}
}
