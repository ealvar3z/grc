package eval

import (
	"strings"
	"testing"

	"grc/internal/parse"
)

func TestExpandConcatMismatch(t *testing.T) {
	env := NewEnv(nil)
	env.Set("x", []string{"a", "b"})
	env.Set("y", []string{"1", "2", "3"})
	ast, err := parse.Parse(strings.NewReader("echo $x^$y\n"))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	concat := parse.FindFirstKind(ast, parse.KConcat)
	if concat == nil {
		t.Fatalf("expected KConcat node")
	}
	_, err = ExpandWord(concat, env)
	if err == nil {
		t.Fatalf("expected concat length mismatch error")
	}
}
