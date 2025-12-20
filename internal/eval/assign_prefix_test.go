package eval

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"grc/internal/parse"
)

func TestAssignPrefixLocalScope(t *testing.T) {
	if !haveCmd(t, "printf") {
		t.Skip("printf not available")
	}
	env := NewEnv(nil)
	input := "x=world printf %s $x\nprintf %s $x\n"
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
	if out.String() != "world" {
		t.Fatalf("unexpected stdout: %q", out.String())
	}
	if vals := env.Get("x"); len(vals) != 0 {
		t.Fatalf("expected x to be unset, got %v", vals)
	}
}

func TestAssignPrefixList(t *testing.T) {
	if !haveCmd(t, "printf") {
		t.Skip("printf not available")
	}
	env := NewEnv(nil)
	input := "x=(a b) printf %s $x\n"
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
	if out.String() != "ab" {
		t.Fatalf("unexpected stdout: %q", out.String())
	}
	if vals := env.Get("x"); len(vals) != 0 {
		t.Fatalf("expected x to be unset, got %v", vals)
	}
}

func TestAssignPrefixEmpty(t *testing.T) {
	if !haveCmd(t, "printf") {
		t.Skip("printf not available")
	}
	env := NewEnv(nil)
	input := "x=() printf %s $x\n"
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
	if out.String() != "" {
		t.Fatalf("unexpected stdout: %q", out.String())
	}
	if vals := env.Get("x"); len(vals) != 0 {
		t.Fatalf("expected x to be unset, got %v", vals)
	}
}
