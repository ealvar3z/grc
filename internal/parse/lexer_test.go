package parse

import (
	"strings"
	"testing"
)

type tokPair struct {
	tok     int
	text    string
	hasText bool
}

func lexPairs(input string) []tokPair {
	lx := NewLexer(strings.NewReader(input))
	var out []tokPair
	for {
		var lval grcSymType
		tok := lx.Lex(&lval)
		if tok == 0 {
			break
		}
		pair := tokPair{tok: tok}
		if lval.node != nil {
			pair.text = lval.node.Tok
			pair.hasText = true
		}
		out = append(out, pair)
	}
	return out
}

func assertTokens(t *testing.T, input string, want []tokPair) {
	t.Helper()
	got := lexPairs(input)
	if len(got) != len(want) {
		t.Fatalf("token count mismatch: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].tok != want[i].tok {
			t.Fatalf("token %d mismatch: got %v, want %v", i, got[i].tok, want[i].tok)
		}
		if want[i].hasText {
			if !got[i].hasText {
				t.Fatalf("token %d missing text", i)
			}
			if got[i].text != want[i].text {
				t.Fatalf("token %d text mismatch: got %q, want %q", i, got[i].text, want[i].text)
			}
		}
	}
}

func TestLexerOperators(t *testing.T) {
	input := "a&&b||c\n"
	want := []tokPair{
		{tok: WORD, text: "a", hasText: true},
		{tok: ANDAND},
		{tok: WORD, text: "b", hasText: true},
		{tok: OROR},
		{tok: WORD, text: "c", hasText: true},
		{tok: int('\n')},
	}
	assertTokens(t, input, want)
}

func TestLexerGreedyRedir(t *testing.T) {
	input := "echo hi>>out\n"
	want := []tokPair{
		{tok: WORD, text: "echo", hasText: true},
		{tok: WORD, text: "hi", hasText: true},
		{tok: REDIR, text: ">>", hasText: true},
		{tok: WORD, text: "out", hasText: true},
		{tok: int('\n')},
	}
	assertTokens(t, input, want)
}

func TestLexerWordBoundaries(t *testing.T) {
	input := "a;b\n"
	want := []tokPair{
		{tok: WORD, text: "a", hasText: true},
		{tok: int(';')},
		{tok: WORD, text: "b", hasText: true},
		{tok: int('\n')},
	}
	assertTokens(t, input, want)
}

func TestLexerKeywords(t *testing.T) {
	input := "fn x { echo hi }\n"
	want := []tokPair{
		{tok: FN, text: "fn", hasText: true},
		{tok: WORD, text: "x", hasText: true},
		{tok: int('{')},
		{tok: WORD, text: "echo", hasText: true},
		{tok: WORD, text: "hi", hasText: true},
		{tok: int('}')},
		{tok: int('\n')},
	}
	assertTokens(t, input, want)
}

func TestLexerSingleQuoted(t *testing.T) {
	input := "echo 'a b; c' x\n"
	want := []tokPair{
		{tok: WORD, text: "echo", hasText: true},
		{tok: WORD, text: "a b; c", hasText: true},
		{tok: WORD, text: "x", hasText: true},
		{tok: int('\n')},
	}
	assertTokens(t, input, want)
}

func TestLexerSingleQuoteEscape(t *testing.T) {
	input := "echo 'a''b' x\n"
	want := []tokPair{
		{tok: WORD, text: "echo", hasText: true},
		{tok: WORD, text: "a'b", hasText: true},
		{tok: WORD, text: "x", hasText: true},
		{tok: int('\n')},
	}
	assertTokens(t, input, want)
}

func TestLexerComment(t *testing.T) {
	input := "echo hi#there\n"
	want := []tokPair{
		{tok: WORD, text: "echo", hasText: true},
		{tok: WORD, text: "hi", hasText: true},
		{tok: int('\n')},
	}
	assertTokens(t, input, want)
}

func TestLexerCommentInQuote(t *testing.T) {
	input := "echo '#'\n"
	want := []tokPair{
		{tok: WORD, text: "echo", hasText: true},
		{tok: WORD, text: "#", hasText: true},
		{tok: int('\n')},
	}
	assertTokens(t, input, want)
}

func TestLexerBackslashNewline(t *testing.T) {
	input := "echo a\\\n b\n"
	want := []tokPair{
		{tok: WORD, text: "echo", hasText: true},
		{tok: WORD, text: "a", hasText: true},
		{tok: WORD, text: "b", hasText: true},
		{tok: int('\n')},
	}
	assertTokens(t, input, want)
}

func TestLexerFreeCaretDollar(t *testing.T) {
	input := "-$x\n"
	want := []tokPair{
		{tok: WORD, text: "-", hasText: true},
		{tok: int('^')},
		{tok: int('$')},
		{tok: WORD, text: "x", hasText: true},
		{tok: int('\n')},
	}
	assertTokens(t, input, want)
}
