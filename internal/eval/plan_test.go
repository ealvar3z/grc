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

func TestPlanDollarSingle(t *testing.T) {
	env := NewEnv(nil)
	env.Set("x", []string{"hi"})
	ast, err := parse.Parse(strings.NewReader("echo $x\n"))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	plan, err := BuildPlan(ast, env)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	if len(plan.Argv) != 2 || plan.Argv[0] != "echo" || plan.Argv[1] != "hi" {
		t.Fatalf("unexpected argv: %v", plan.Argv)
	}
}

func TestPlanDollarList(t *testing.T) {
	env := NewEnv(nil)
	env.Set("x", []string{"a", "b"})
	ast, err := parse.Parse(strings.NewReader("echo $x\n"))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	plan, err := BuildPlan(ast, env)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	if len(plan.Argv) != 3 || plan.Argv[0] != "echo" || plan.Argv[1] != "a" || plan.Argv[2] != "b" {
		t.Fatalf("unexpected argv: %v", plan.Argv)
	}
}

func TestPlanDollarConcatTwoVars(t *testing.T) {
	env := NewEnv(nil)
	env.Set("x", []string{"a", "b"})
	env.Set("y", []string{"1", "2"})
	ast, err := parse.Parse(strings.NewReader("echo $x^$y\n"))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	plan, err := BuildPlan(ast, env)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	want := []string{"echo", "a1", "b2"}
	if len(plan.Argv) != len(want) {
		t.Fatalf("unexpected argv: %v", plan.Argv)
	}
	for i := range want {
		if plan.Argv[i] != want[i] {
			t.Fatalf("unexpected argv: %v", plan.Argv)
		}
	}
}

func TestPlanDollarConcatVarAndLiteral(t *testing.T) {
	env := NewEnv(nil)
	env.Set("x", []string{"a", "b"})
	ast, err := parse.Parse(strings.NewReader("echo $x^y\n"))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	plan, err := BuildPlan(ast, env)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	want := []string{"echo", "ay", "by"}
	if len(plan.Argv) != len(want) {
		t.Fatalf("unexpected argv: %v", plan.Argv)
	}
	for i := range want {
		if plan.Argv[i] != want[i] {
			t.Fatalf("unexpected argv: %v", plan.Argv)
		}
	}
}

func TestPlanFreeCaretDollar(t *testing.T) {
	env := NewEnv(nil)
	env.Set("x", []string{"O2"})
	ast, err := parse.Parse(strings.NewReader("echo -$x\n"))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	plan, err := BuildPlan(ast, env)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	want := []string{"echo", "-O2"}
	if len(plan.Argv) != len(want) {
		t.Fatalf("unexpected argv: %v", plan.Argv)
	}
	for i := range want {
		if plan.Argv[i] != want[i] {
			t.Fatalf("unexpected argv: %v", plan.Argv)
		}
	}
}

func TestPlanFreeCaretSuffix(t *testing.T) {
	env := NewEnv(nil)
	env.Set("stem", []string{"foo"})
	ast, err := parse.Parse(strings.NewReader("echo $stem.c\n"))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	plan, err := BuildPlan(ast, env)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	want := []string{"echo", "foo.c"}
	if len(plan.Argv) != len(want) {
		t.Fatalf("unexpected argv: %v", plan.Argv)
	}
	for i := range want {
		if plan.Argv[i] != want[i] {
			t.Fatalf("unexpected argv: %v", plan.Argv)
		}
	}
}

func TestPlanNoFreeCaretWithSpace(t *testing.T) {
	env := NewEnv(nil)
	env.Set("x", []string{"O2"})
	ast, err := parse.Parse(strings.NewReader("echo - $x\n"))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	plan, err := BuildPlan(ast, env)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	want := []string{"echo", "-", "O2"}
	if len(plan.Argv) != len(want) {
		t.Fatalf("unexpected argv: %v", plan.Argv)
	}
	for i := range want {
		if plan.Argv[i] != want[i] {
			t.Fatalf("unexpected argv: %v", plan.Argv)
		}
	}
}

func TestPlanSeq(t *testing.T) {
	ast, err := parse.Parse(strings.NewReader("a;b\n"))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	plan, err := BuildPlan(ast, NewEnv(nil))
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	if plan == nil || plan.Next == nil {
		t.Fatalf("expected Next plan")
	}
	if len(plan.Argv) == 0 || plan.Argv[0] != "a" {
		t.Fatalf("unexpected left argv: %v", plan.Argv)
	}
	if len(plan.Next.Argv) == 0 || plan.Next.Argv[0] != "b" {
		t.Fatalf("unexpected right argv: %v", plan.Next.Argv)
	}
}

func TestPlanBackground(t *testing.T) {
	ast, err := parse.Parse(strings.NewReader("a&\n"))
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
	if !plan.Background {
		t.Fatalf("expected Background=true")
	}
	if len(plan.Argv) == 0 || plan.Argv[0] != "a" {
		t.Fatalf("unexpected argv: %v", plan.Argv)
	}
}

func TestPlanAnd(t *testing.T) {
	ast, err := parse.Parse(strings.NewReader("a&&b\n"))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	plan, err := BuildPlan(ast, NewEnv(nil))
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	if plan == nil || plan.IfOK == nil {
		t.Fatalf("expected IfOK plan")
	}
	if len(plan.Argv) == 0 || plan.Argv[0] != "a" {
		t.Fatalf("unexpected left argv: %v", plan.Argv)
	}
	if len(plan.IfOK.Argv) == 0 || plan.IfOK.Argv[0] != "b" {
		t.Fatalf("unexpected right argv: %v", plan.IfOK.Argv)
	}
}

func TestPlanOr(t *testing.T) {
	ast, err := parse.Parse(strings.NewReader("a||b\n"))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	plan, err := BuildPlan(ast, NewEnv(nil))
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	if plan == nil || plan.IfFail == nil {
		t.Fatalf("expected IfFail plan")
	}
	if len(plan.Argv) == 0 || plan.Argv[0] != "a" {
		t.Fatalf("unexpected left argv: %v", plan.Argv)
	}
	if len(plan.IfFail.Argv) == 0 || plan.IfFail.Argv[0] != "b" {
		t.Fatalf("unexpected right argv: %v", plan.IfFail.Argv)
	}
}
