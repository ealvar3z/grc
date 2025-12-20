package eval

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"grc/internal/parse"
)

func TestRunSimple(t *testing.T) {
	if !haveCmd(t, "printf") {
		t.Skip("printf not available")
	}
	ast, err := parse.Parse(strings.NewReader("printf hi\n"))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	plan, err := BuildPlan(ast, NewEnv(nil))
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	var out bytes.Buffer
	res := (&Runner{}).RunPlan(plan, strings.NewReader(""), &out, io.Discard)
	if res.Status != 0 {
		t.Fatalf("expected status 0, got %d", res.Status)
	}
	if out.String() != "hi" {
		t.Fatalf("unexpected stdout: %q", out.String())
	}
}

func TestRunPipe(t *testing.T) {
	if !haveCmd(t, "printf") || !haveCmd(t, "wc") {
		t.Skip("printf or wc not available")
	}
	ast, err := parse.Parse(strings.NewReader("printf 'hi\n'|wc -c\n"))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	plan, err := BuildPlan(ast, NewEnv(nil))
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	var out bytes.Buffer
	res := (&Runner{}).RunPlan(plan, strings.NewReader(""), &out, io.Discard)
	if res.Status != 0 {
		t.Fatalf("expected status 0, got %d", res.Status)
	}
	got := strings.TrimSpace(out.String())
	if got != "3" {
		t.Fatalf("unexpected stdout: %q", out.String())
	}
}

func TestRunRedirOut(t *testing.T) {
	if !haveCmd(t, "printf") {
		t.Skip("printf not available")
	}
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out")
	ast, err := parse.Parse(strings.NewReader(fmt.Sprintf("printf hi > %s\n", outPath)))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	plan, err := BuildPlan(ast, NewEnv(nil))
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	res := (&Runner{}).RunPlan(plan, strings.NewReader(""), io.Discard, io.Discard)
	if res.Status != 0 {
		t.Fatalf("expected status 0, got %d", res.Status)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(data) != "hi" {
		t.Fatalf("unexpected file contents: %q", string(data))
	}
}

func TestRunSeq(t *testing.T) {
	if !haveCmd(t, "printf") {
		t.Skip("printf not available")
	}
	ast, err := parse.Parse(strings.NewReader("printf a;printf b\n"))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	plan, err := BuildPlan(ast, NewEnv(nil))
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	var out bytes.Buffer
	res := (&Runner{}).RunPlan(plan, strings.NewReader(""), &out, io.Discard)
	if res.Status != 0 {
		t.Fatalf("expected status 0, got %d", res.Status)
	}
	if out.String() != "ab" {
		t.Fatalf("unexpected stdout: %q", out.String())
	}
}

func TestRunAndOr(t *testing.T) {
	if !haveCmd(t, "sh") || !haveCmd(t, "printf") {
		t.Skip("sh or printf not available")
	}
	input := "sh -c 'exit 1'&&printf x; sh -c 'exit 1'||printf y\n"
	ast, err := parse.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	plan, err := BuildPlan(ast, NewEnv(nil))
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	var out bytes.Buffer
	res := (&Runner{}).RunPlan(plan, strings.NewReader(""), &out, io.Discard)
	if res.Status != 0 {
		t.Fatalf("expected status 0, got %d", res.Status)
	}
	if out.String() != "y" {
		t.Fatalf("unexpected stdout: %q", out.String())
	}
}

func TestRunDollarSingle(t *testing.T) {
	if !haveCmd(t, "printf") {
		t.Skip("printf not available")
	}
	env := NewEnv(nil)
	env.Set1("x", "hi")
	ast, err := parse.Parse(strings.NewReader("printf %s $x\n"))
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
}

func TestRunDollarListArgs(t *testing.T) {
	if !haveCmd(t, "printf") {
		t.Skip("printf not available")
	}
	env := NewEnv(nil)
	env.Set("x", []string{"a", "b"})
	ast, err := parse.Parse(strings.NewReader("printf %s $x\n"))
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

func TestRunDollarConcat(t *testing.T) {
	if !haveCmd(t, "printf") {
		t.Skip("printf not available")
	}
	env := NewEnv(nil)
	env.Set("x", []string{"a", "b"})
	env.Set("y", []string{"1", "2"})
	ast, err := parse.Parse(strings.NewReader("printf %s $x^y\n"))
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
	if out.String() != "a1a2b1b2" {
		t.Fatalf("unexpected stdout: %q", out.String())
	}
}

func haveCmd(t *testing.T, name string) bool {
	_, err := exec.LookPath(name)
	if err != nil {
		return false
	}
	return true
}
