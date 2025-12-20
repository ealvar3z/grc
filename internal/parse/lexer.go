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
	eof  bool

	peeked bool
	peek   lexRune

	pendingTok   int
	pendingVal   *Node
	prevConcat   bool
	prevWasDollar bool
	sawSpace     bool
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
	if lx.pendingTok != 0 {
		tok := lx.pendingTok
		lx.pendingTok = 0
		if lx.pendingVal != nil {
			lval.node = lx.pendingVal
			lx.pendingVal = nil
		}
		lx.prevConcat = canConcatToken(tok)
		lx.prevWasDollar = tok == int('$')
		lx.sawSpace = false
		return tok
	}
	for {
		r, line, col, err := lx.readRune()
		if err != nil {
			return 0
		}

		switch r {
		case ' ', '\t':
			lx.sawSpace = true
			if lx.prevWasDollar {
				lx.prevWasDollar = false
			}
			continue
		case '\n':
			lx.prevConcat = false
			lx.prevWasDollar = false
			lx.sawSpace = false
			return int(r)
		case '&':
			if lx.consumeIf('&') {
				return lx.emitToken(ANDAND, nil, lval)
			}
			return lx.emitToken(int(r), nil, lval)
		case '|':
			if lx.consumeIf('|') {
				return lx.emitToken(OROR, nil, lval)
			}
			return lx.emitToken(int(r), nil, lval)
		case '>':
			if lx.consumeIf('>') {
				node := &Node{Kind: KRedir, Tok: ">>", Pos: Pos{Line: line, Col: col}}
				return lx.emitToken(REDIR, node, lval)
			}
			node := &Node{Kind: KRedir, Tok: ">", Pos: Pos{Line: line, Col: col}}
			return lx.emitToken(REDIR, node, lval)
		case '<':
			if lx.consumeIf('>') {
				node := &Node{Kind: KRedir, Tok: "<>", Pos: Pos{Line: line, Col: col}}
				return lx.emitToken(REDIR, node, lval)
			}
			node := &Node{Kind: KRedir, Tok: "<", Pos: Pos{Line: line, Col: col}}
			return lx.emitToken(REDIR, node, lval)
		case '\'':
			text, ok := lx.readSingleQuoted()
			if !ok {
				lx.Error("unterminated quote")
				return 0
			}
			node := W(text)
			node.Pos = Pos{Line: line, Col: col}
			return lx.emitToken(WORD, node, lval)
		case ';', '(', ')', '{', '}', '=', '^', '$', '"', '`':
			return lx.emitToken(int(r), nil, lval)
		default:
			word := ""
			if lx.prevWasDollar && !lx.sawSpace && isIdentRune(r) {
				word = lx.readIdentTail(r)
			} else {
				word = lx.readWordTail(r)
			}
			if word == "" {
				return 0
			}
			node := W(word)
			node.Pos = Pos{Line: line, Col: col}
			if tok, ok := keywordToken(word); ok {
				return lx.emitToken(tok, node, lval)
			}
			return lx.emitToken(WORD, node, lval)
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

func (lx *Lexer) readIdentTail(first rune) string {
	var b strings.Builder
	b.WriteRune(first)
	for {
		r, _, _, err := lx.peekRune()
		if err != nil {
			break
		}
		if !isIdentRune(r) {
			break
		}
		r, _, _, _ = lx.readRune()
		b.WriteRune(r)
	}
	return b.String()
}

func isWordBreak(r rune) bool {
	switch r {
	case ' ', '\t', '\n', ';', '&', '|', '(', ')', '{', '}', '=', '^', '$', '"', '\'', '`', '<', '>':
		return true
	default:
		return false
	}
}

func isIdentRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z':
		return true
	case r >= 'A' && r <= 'Z':
		return true
	case r >= '0' && r <= '9':
		return true
	case r == '_':
		return true
	case r == '*':
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
		if errors.Is(err, io.EOF) {
			lx.eof = true
		}
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

func (lx *Lexer) readSingleQuoted() (string, bool) {
	var b strings.Builder
	for {
		r, _, _, err := lx.readRune()
		if err != nil {
			return "", false
		}
		if r == '\'' {
			return b.String(), true
		}
		b.WriteRune(r)
	}
}

// EOF reports whether the lexer has reached end of input.
func (lx *Lexer) EOF() bool {
	return lx.eof
}

func (lx *Lexer) emitToken(tok int, node *Node, lval *grcSymType) int {
	if tok == int('$') {
		if !lx.sawSpace && lx.prevConcat {
			lx.pendingTok = tok
			lx.pendingVal = nil
			lx.prevConcat = false
			lx.prevWasDollar = false
			return int('^')
		}
		lx.prevConcat = false
		lx.prevWasDollar = true
		lx.sawSpace = false
		return tok
	}
	if lx.prevWasDollar && !lx.sawSpace {
		if node != nil {
			lval.node = node
		}
		lx.prevWasDollar = false
		lx.prevConcat = canConcatTokenAfterDollar(tok)
		lx.sawSpace = false
		return tok
	}
	if !lx.sawSpace && lx.prevConcat && canConcatToken(tok) {
		lx.pendingTok = tok
		lx.pendingVal = node
		lx.prevConcat = false
		lx.prevWasDollar = false
		return int('^')
	}
	if node != nil {
		lval.node = node
	}
	lx.prevConcat = canConcatToken(tok)
	lx.prevWasDollar = false
	lx.sawSpace = false
	return tok
}

func canConcatToken(tok int) bool {
	switch tok {
	case WORD, int('`'), int('"'):
		return true
	default:
		return false
	}
}

func canConcatTokenAfterDollar(tok int) bool {
	switch tok {
	case WORD, int('`'), int('"'):
		return true
	default:
		return false
	}
}
