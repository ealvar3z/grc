package eval

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"grc/internal/parse"
)

func TestDotBuiltin(t *testing.T) {
	if !haveCmd(t, "printf") {
		t.Skip("printf not available")
	}
	dir := t.TempDir()
	script := "printf hi\n"
	path := filepath.Join(dir, "script.rc")
	if err := os.WriteFile(path, []byte(script), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	input := ". " + path + "\n"
	ast, err := parse.ParseAll(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseAll error: %v", err)
	}
	env := NewEnv(nil)
	plan, err := BuildPlan(ast, env)
	if err != nil {
		t.Fatalf("BuildPlan error: %v", err)
	}
	var out bytes.Buffer
	res := (&Runner{Env: env}).RunPlan(plan, strings.NewReader(""), &out, os.Stderr)
	if res.Status != 0 {
		t.Fatalf("expected status 0, got %d", res.Status)
	}
	if out.String() != "hi" {
		t.Fatalf("unexpected stdout: %q", out.String())
	}
}
