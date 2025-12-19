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
