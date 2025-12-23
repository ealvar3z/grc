package eval

import (
	"bytes"
	"strings"
	"testing"

	"grc/internal/parse"
)

func TestSubshellIsolation(t *testing.T) {
	if !haveCmd(t, "printf") {
		t.Skip("printf not available")
	}
	env := NewEnv(nil)
	env.Set1("x", "outer")
	input := "@ { x=inner; printf %s $x }\nprintf %s $x\n"
	ast, err := parse.ParseAll(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseAll returned error: %v", err)
	}
	plan, err := BuildPlan(ast, env)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	var out bytes.Buffer
	runner := &Runner{Env: env}
	res := runner.RunPlan(plan, strings.NewReader(""), &out, &out)
	if res.Status != 0 {
		t.Fatalf("expected status 0, got %d", res.Status)
	}
	if out.String() != "innerouter" {
		t.Fatalf("unexpected stdout: %q", out.String())
	}
	if got := env.Get("x"); len(got) != 1 || got[0] != "outer" {
		t.Fatalf("env leaked from subshell: %v", got)
	}
}
