package parse

import (
	"bufio"
	"errors"
	"io"
	"strings"
)

// Lexer provides tokens for the generated parser.
type Lexer struct {
	r   *bufio.Reader
	Err error

	line int
	col  int

	peeked bool
	peek   lexRune
}

type lexRune struct {
	r        rune
	line     int
	col      int
	nextLine int
	nextCol  int
	err      error
}

func NewLexer(rd io.Reader) *Lexer {
	return &Lexer{r: bufio.NewReader(rd), line: 1}
}

func (lx *Lexer) Lex(lval *grcSymType) int {
	for {
		r, line, col, err := lx.readRune()
		if err != nil {
			return 0
		}

		switch r {
		case ' ', '\t':
			continue
		case '\n':
			return int(r)
		case '&':
			if lx.consumeIf('&') {
				return ANDAND
			}
			return int(r)
		case '|':
			if lx.consumeIf('|') {
				return OROR
			}
			return int(r)
		case '>':
			if lx.consumeIf('>') {
				lval.node = &Node{Kind: KRedir, Tok: ">>", Pos: Pos{Line: line, Col: col}}
				return redirwToken()
			}
			lval.node = &Node{Kind: KRedir, Tok: ">", Pos: Pos{Line: line, Col: col}}
			return REDIR
		case '<':
			if lx.consumeIf('>') {
				lval.node = &Node{Kind: KRedir, Tok: "<>", Pos: Pos{Line: line, Col: col}}
				return REDIR
			}
			lval.node = &Node{Kind: KRedir, Tok: "<", Pos: Pos{Line: line, Col: col}}
			return REDIR
		case ';', '(', ')', '{', '}', '=', '^', '$', '"', '`':
			return int(r)
		default:
			word := lx.readWordTail(r)
			if word == "" {
				return 0
			}
			node := W(word)
			node.Pos = Pos{Line: line, Col: col}
			if tok, ok := keywordToken(word); ok {
				lval.node = node
				return tok
			}
			lval.node = node
			return WORD
		}
	}
}

func (lx *Lexer) Error(s string) {
	if lx.Err == nil {
		lx.Err = errors.New(s)
	}
}

func keywordToken(word string) (int, bool) {
	switch word {
	case "for":
		return FOR, true
	case "in":
		return IN, true
	case "while":
		return WHILE, true
	case "if":
		return IF, true
	case "not":
		return NOT, true
	case "fn":
		return FN, true
	case "switch":
		return SWITCH, true
	default:
		return 0, false
	}
}

func (lx *Lexer) readWordTail(first rune) string {
	var b strings.Builder
	b.WriteRune(first)
	for {
		r, _, _, err := lx.peekRune()
		if err != nil {
			break
		}
		if isWordBreak(r) {
			break
		}
		r, _, _, _ = lx.readRune()
		b.WriteRune(r)
	}
	return b.String()
}

func isWordBreak(r rune) bool {
	switch r {
	case ' ', '\t', '\n', ';', '&', '|', '(', ')', '{', '}', '=', '^', '$', '"', '`', '<', '>':
		return true
	default:
		return false
	}
}

func (lx *Lexer) consumeIf(want rune) bool {
	r, _, _, err := lx.peekRune()
	if err != nil {
		return false
	}
	if r != want {
		return false
	}
	_, _, _, _ = lx.readRune()
	return true
}

func (lx *Lexer) readRune() (rune, int, int, error) {
	if lx.peeked {
		pr := lx.peek
		lx.peeked = false
		lx.line = pr.nextLine
		lx.col = pr.nextCol
		return pr.r, pr.line, pr.col, pr.err
	}
	pr := lx.readRawRune()
	lx.line = pr.nextLine
	lx.col = pr.nextCol
	return pr.r, pr.line, pr.col, pr.err
}

func (lx *Lexer) peekRune() (rune, int, int, error) {
	if lx.peeked {
		return lx.peek.r, lx.peek.line, lx.peek.col, lx.peek.err
	}
	lx.peek = lx.readRawRune()
	lx.peeked = true
	return lx.peek.r, lx.peek.line, lx.peek.col, lx.peek.err
}

func (lx *Lexer) readRawRune() lexRune {
	r, _, err := lx.r.ReadRune()
	if err != nil {
		return lexRune{r: 0, err: err, line: lx.line, col: lx.col, nextLine: lx.line, nextCol: lx.col}
	}
	line := lx.line
	col := lx.col + 1
	nextLine := line
	nextCol := col
	if r == '\n' {
		nextLine = line + 1
		nextCol = 0
	}
	return lexRune{r: r, line: line, col: col, nextLine: nextLine, nextCol: nextCol}
}

func redirwToken() int {
	return REDIR
}
