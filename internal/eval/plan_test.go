package eval

import (
	"strings"
	"testing"

	"grc/internal/parse"
)

func TestPlanSimpleCall(t *testing.T) {
	ast, err := parse.Parse(strings.NewReader("echo hi\n"))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	plan, err := BuildPlan(ast, NewEnv(nil))
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	if plan == nil {
		t.Fatalf("expected non-nil plan")
	}
	if len(plan.Argv) < 2 || plan.Argv[0] != "echo" || plan.Argv[1] != "hi" {
		t.Fatalf("unexpected argv: %v", plan.Argv)
	}
}

func TestPlanConcat(t *testing.T) {
	ast, err := parse.Parse(strings.NewReader("echo a^b\n"))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	plan, err := BuildPlan(ast, NewEnv(nil))
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	if plan == nil {
		t.Fatalf("expected non-nil plan")
	}
	if len(plan.Argv) < 2 || plan.Argv[1] != "ab" {
		t.Fatalf("unexpected argv: %v", plan.Argv)
	}
}

func TestPlanRedirOut(t *testing.T) {
	ast, err := parse.Parse(strings.NewReader("echo hi > out\n"))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	plan, err := BuildPlan(ast, NewEnv(nil))
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	if plan == nil {
		t.Fatalf("expected non-nil plan")
	}
	if len(plan.Redirs) != 1 {
		t.Fatalf("expected 1 redir, got %d", len(plan.Redirs))
	}
	if plan.Redirs[0].Op != ">" {
		t.Fatalf("unexpected redir op: %q", plan.Redirs[0].Op)
	}
	if len(plan.Redirs[0].Target) != 1 || plan.Redirs[0].Target[0] != "out" {
		t.Fatalf("unexpected redir target: %v", plan.Redirs[0].Target)
	}
}

func TestPlanPipe(t *testing.T) {
	ast, err := parse.Parse(strings.NewReader("a|b\n"))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	plan, err := BuildPlan(ast, NewEnv(nil))
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	if plan == nil || plan.PipeTo == nil {
		t.Fatalf("expected pipe plan")
	}
	if len(plan.Argv) == 0 || plan.Argv[0] != "a" {
		t.Fatalf("unexpected left argv: %v", plan.Argv)
	}
	if len(plan.PipeTo.Argv) == 0 || plan.PipeTo.Argv[0] != "b" {
		t.Fatalf("unexpected right argv: %v", plan.PipeTo.Argv)
	}
}
