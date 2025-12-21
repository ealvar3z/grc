package eval

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"grc/internal/parse"
)

func TestTraceBuiltinPwd(t *testing.T) {
	env := NewEnv(nil)
	ast, err := parse.ParseAll(strings.NewReader("pwd\n"))
	if err != nil {
		t.Fatalf("ParseAll returned error: %v", err)
	}
	plan, err := BuildPlan(ast, env)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	var trace bytes.Buffer
	runner := &Runner{Env: env, Trace: true, TraceWriter: &trace}
	runner.RunPlan(plan, strings.NewReader(""), io.Discard, io.Discard)
	if !strings.Contains(trace.String(), "+ pwd") {
		t.Fatalf("unexpected trace: %q", trace.String())
	}
}
