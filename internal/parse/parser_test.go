package parse

import (
	"strings"
	"testing"
)

func TestParseReadsInput(t *testing.T) {
	input := "echo hi\n"
	node, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if node == nil {
		t.Fatalf("expected non-nil AST")
	}
	root := node
	if root.Kind == KSeq && root.Left != nil {
		root = root.Left
	}
	if root.Kind != KCall {
		t.Fatalf("expected call node, got %#v", root)
	}
	words := collectWords(root)
	if len(words) < 2 || words[0] != "echo" || words[1] != "hi" {
		t.Fatalf("expected words [echo hi], got %v", words)
	}
}

func TestParseQuotedArg(t *testing.T) {
	input := "echo 'a b' x\n"
	node, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if node == nil {
		t.Fatalf("expected non-nil AST")
	}
	words := collectWords(node)
	if len(words) < 3 || words[0] != "echo" || words[1] != "a b" || words[2] != "x" {
		t.Fatalf("expected words [echo a b x], got %v", words)
	}
}

func TestParseSequence(t *testing.T) {
	input := "a;b\n"
	node, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if node == nil {
		t.Fatalf("expected non-nil AST")
	}
	if node.Kind != KSeq || node.Left == nil || node.Right == nil {
		t.Fatalf("expected sequence node, got %#v", node)
	}
}

func TestParsePipe(t *testing.T) {
	input := "a|b\n"
	node, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if node == nil {
		t.Fatalf("expected non-nil AST")
	}
	if node.Kind != KPipe {
		t.Fatalf("expected pipe node, got %#v", node)
	}
}

func collectWords(n *Node) []string {
	if n == nil {
		return nil
	}
	var out []string
	if n.Kind == KWord && n.Tok != "" {
		out = append(out, n.Tok)
	}
	out = append(out, collectWords(n.Left)...)
	out = append(out, collectWords(n.Right)...)
	for _, child := range n.List {
		out = append(out, collectWords(child)...)
	}
	return out
}
