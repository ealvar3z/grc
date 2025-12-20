package eval

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"grc/internal/parse"
)

func TestBackquoteSimple(t *testing.T) {
	if !haveCmd(t, "printf") {
		t.Skip("printf not available")
	}
	env := NewEnv(nil)
	input := "printf %s `{ printf hi }\n"
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
	if out.String() != "hi" {
		t.Fatalf("unexpected stdout: %q", out.String())
	}
}

func TestBackquoteWordSplit(t *testing.T) {
	if !haveCmd(t, "printf") {
		t.Skip("printf not available")
	}
	env := NewEnv(nil)
	input := "printf %s `{ printf 'a b' }\n"
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
}

func TestBackquoteConcat(t *testing.T) {
	if !haveCmd(t, "printf") {
		t.Skip("printf not available")
	}
	env := NewEnv(nil)
	input := "x=pre\nprintf %s $x^`{ printf fix }\n"
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
	if out.String() != "prefix" {
		t.Fatalf("unexpected stdout: %q", out.String())
	}
}

func TestBackquoteStatus(t *testing.T) {
	if !haveCmd(t, "sh") || !haveCmd(t, "printf") {
		t.Skip("sh or printf not available")
	}
	env := NewEnv(nil)
	input := "printf %s `{ sh -c 'exit 7' } $status\n"
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
	if out.String() != "7" {
		t.Fatalf("unexpected stdout: %q", out.String())
	}
}
