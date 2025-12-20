package eval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"grc/internal/parse"
)

func TestGlobMatches(t *testing.T) {
	dir := t.TempDir()
	files := []string{"a1.txt", "a2.txt", "b.txt"}
	for _, name := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer os.Chdir(old)
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	ast, err := parse.Parse(strings.NewReader("echo a*.txt\n"))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	plan, err := BuildPlan(ast, NewEnv(nil))
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	want := []string{"echo", "a1.txt", "a2.txt"}
	if len(plan.Argv) != len(want) {
		t.Fatalf("unexpected argv: %v", plan.Argv)
	}
	for i := range want {
		if plan.Argv[i] != want[i] {
			t.Fatalf("unexpected argv: %v", plan.Argv)
		}
	}
}

func TestGlobNoMatchesLiteral(t *testing.T) {
	ast, err := parse.Parse(strings.NewReader("echo z*.txt\n"))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	plan, err := BuildPlan(ast, NewEnv(nil))
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	want := []string{"echo", "z*.txt"}
	if len(plan.Argv) != len(want) {
		t.Fatalf("unexpected argv: %v", plan.Argv)
	}
	for i := range want {
		if plan.Argv[i] != want[i] {
			t.Fatalf("unexpected argv: %v", plan.Argv)
		}
	}
}

func TestGlobFromDollarConcat(t *testing.T) {
	dir := t.TempDir()
	files := []string{"a1.txt", "a2.txt", "b.txt"}
	for _, name := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer os.Chdir(old)
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	env := NewEnv(nil)
	env.Set("stem", []string{"a"})
	ast, err := parse.Parse(strings.NewReader("echo $stem^*.txt\n"))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	plan, err := BuildPlan(ast, env)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	want := []string{"echo", "a1.txt", "a2.txt"}
	if len(plan.Argv) != len(want) {
		t.Fatalf("unexpected argv: %v", plan.Argv)
	}
	for i := range want {
		if plan.Argv[i] != want[i] {
			t.Fatalf("unexpected argv: %v", plan.Argv)
		}
	}
}
