package eval

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"grc/internal/parse"
)

func TestScripts(t *testing.T) {
	if _, err := exec.LookPath("printf"); err != nil {
		t.Skip("printf not available")
	}
	paths, err := filepath.Glob(filepath.Join("testdata", "scripts", "*.rc"))
	if err != nil {
		t.Fatalf("glob scripts: %v", err)
	}
	for _, path := range paths {
		base := strings.TrimSuffix(path, ".rc")
		name := filepath.Base(base)
		rc, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read script: %v", err)
		}
		wantOut, err := os.ReadFile(base + ".out")
		if err != nil {
			t.Fatalf("read out: %v", err)
		}
		wantStatusBytes, err := os.ReadFile(base + ".status")
		if err != nil {
			t.Fatalf("read status: %v", err)
		}
		wantStatus := strings.TrimSpace(string(wantStatusBytes))

		t.Run(name, func(t *testing.T) {
			env := NewEnv(nil)
			ast, err := parse.ParseAll(strings.NewReader(string(rc)))
			if err != nil {
				t.Fatalf("ParseAll returned error: %v", err)
			}
			plan, err := BuildPlan(ast, env)
			if err != nil {
				t.Fatalf("BuildPlan returned error: %v", err)
			}
			var out bytes.Buffer
			res := (&Runner{Env: env}).RunPlan(plan, strings.NewReader(""), &out, io.Discard)
			_ = res
			if out.String() != string(wantOut) {
				t.Fatalf("unexpected stdout: %q", out.String())
			}
			gotStatus := ""
			if vals := env.Get("status"); len(vals) > 0 {
				gotStatus = vals[0]
			}
			if gotStatus != wantStatus {
				t.Fatalf("unexpected status: %q", gotStatus)
			}
		})
	}
}
