package parse

import (
	"strings"
	"testing"
)

func TestParseReadsInput(t *testing.T) {
	input := "echo\n"
	node, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if node == nil || node.Kind != KCall {
		t.Fatalf("expected call node, got %#v", node)
	}
	if node.Left == nil || node.Left.Kind != KWord || node.Left.Tok != "echo" {
		t.Fatalf("expected word node, got %#v", node.Left)
	}
}
