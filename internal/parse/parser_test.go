package parse

import (
	"strings"
	"testing"
)

func TestParseSimpleCall(t *testing.T) {
	input := "echo hi\n"
	node, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if node == nil {
		t.Fatalf("expected non-nil AST")
	}
	words := PreorderWords(node)
	if !isSubsequence(words, []string{"echo", "hi"}) {
		t.Fatalf("expected words [echo hi] in order, got %v", words)
	}
}

func TestParseQuotedWord(t *testing.T) {
	input := "echo 'a b; c' x\n"
	node, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if node == nil {
		t.Fatalf("expected non-nil AST")
	}
	words := PreorderWords(node)
	if !isSubsequence(words, []string{"echo", "a b; c", "x"}) {
		t.Fatalf("expected words [echo a b; c x] in order, got %v", words)
	}
}

func TestParseSeq(t *testing.T) {
	input := "a;b\n"
	node, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if node == nil {
		t.Fatalf("expected non-nil AST")
	}
	kinds := KindsPreorder(node)
	if node.Kind != KSeq {
		if countKind(kinds, KSeq) != 1 {
			t.Fatalf("expected exactly one KSeq, got %v", kinds)
		}
	}
	words := PreorderWords(node)
	if !isSubsequence(words, []string{"a", "b"}) {
		t.Fatalf("expected words [a b] in order, got %v", words)
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
	kinds := KindsPreorder(node)
	if countKind(kinds, KPipe) == 0 {
		t.Fatalf("expected KPipe in preorder kinds, got %v", kinds)
	}
	words := PreorderWords(node)
	if !isSubsequence(words, []string{"a", "b"}) {
		t.Fatalf("expected words [a b] in order, got %v", words)
	}
}

func TestParseConcat(t *testing.T) {
	input := "echo a^b\n"
	node, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if node == nil {
		t.Fatalf("expected non-nil AST")
	}
	kinds := KindsPreorder(node)
	if countKind(kinds, KConcat) == 0 {
		t.Fatalf("expected KConcat in preorder kinds, got %v", kinds)
	}
	words := PreorderWords(node)
	if !isSubsequence(words, []string{"echo", "a", "b"}) {
		t.Fatalf("expected words [echo a b] in order, got %v", words)
	}
}

func TestParseAllMultiline(t *testing.T) {
	input := "echo a\n echo b\n"
	node, err := ParseAll(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseAll returned error: %v", err)
	}
	if node == nil {
		t.Fatalf("expected non-nil AST")
	}
	kinds := KindsPreorder(node)
	if countKind(kinds, KSeq) == 0 {
		t.Fatalf("expected KSeq in preorder kinds, got %v", kinds)
	}
	words := PreorderWords(node)
	if !isSubsequence(words, []string{"echo", "a", "echo", "b"}) {
		t.Fatalf("expected words [echo a echo b] in order, got %v", words)
	}
}

func TestParseRedirOutSpaced(t *testing.T) {
	input := "echo hi > out\n"
	node, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if node == nil {
		t.Fatalf("expected non-nil AST")
	}
	redir := FindFirstKind(node, KRedir)
	if redir == nil {
		t.Fatalf("expected KRedir node")
	}
	if redir.Tok != ">" {
		t.Fatalf("expected redir Tok \">\", got %q", redir.Tok)
	}
	words := PreorderWords(node)
	if !isSubsequence(words, []string{"echo", "hi", "out"}) {
		t.Fatalf("expected words [echo hi out] in order, got %v", words)
	}
}

func TestParseRedirOutAdjacent(t *testing.T) {
	input := "echo hi>>out\n"
	node, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if node == nil {
		t.Fatalf("expected non-nil AST")
	}
	redir := FindFirstKind(node, KRedir)
	if redir == nil {
		t.Fatalf("expected KRedir node")
	}
	if redir.Tok != ">>" {
		t.Fatalf("expected redir Tok \">>\", got %q", redir.Tok)
	}
	words := PreorderWords(node)
	if !isSubsequence(words, []string{"echo", "hi", "out"}) {
		t.Fatalf("expected words [echo hi out] in order, got %v", words)
	}
}

func isSubsequence(haystack, needle []string) bool {
	if len(needle) == 0 {
		return true
	}
	j := 0
	for _, s := range haystack {
		if s == needle[j] {
			j++
			if j == len(needle) {
				return true
			}
		}
	}
	return false
}

func countKind(kinds []Kind, want Kind) int {
	count := 0
	for _, k := range kinds {
		if k == want {
			count++
		}
	}
	return count
}
