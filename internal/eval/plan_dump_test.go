package eval

import (
	"strings"
	"testing"

	"grc/internal/parse"
)

func TestDumpPlanContainsLinks(t *testing.T) {
	ast, err := parse.ParseAll(strings.NewReader("a|b;c\n"))
	if err != nil {
		t.Fatalf("ParseAll returned error: %v", err)
	}
	plan, err := BuildPlan(ast, NewEnv(nil))
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	dump := DumpPlan(plan)
	if !strings.Contains(dump, "PIPE") {
		t.Fatalf("expected PIPE in dump, got %q", dump)
	}
	if !strings.Contains(dump, "NEXT") {
		t.Fatalf("expected NEXT in dump, got %q", dump)
	}
}
