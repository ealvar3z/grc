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

	pendingTok    int
	pendingVal    *Node
	prevConcat    bool
	prevWasDollar bool
	sawSpace      bool
	wordState     int
	endSent       bool

	fdLeft  int
	fdRight int
}

type lexRune struct {
	r        rune
	line     int
	col      int
	nextLine int
	nextCol  int
	err      error
}

const (
	wordNW = iota
	wordRW
	wordKW
)

const (
	fdUnset  = -9
	fdClosed = -1
)

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
			if lx.endSent {
				return 0
			}
			lx.endSent = true
			return END
		}

		switch r {
		case '\\':
			next, _, _, err := lx.peekRune()
			if err == nil && next == '\n' {
				_, _, _, _ = lx.readRune()
				lx.sawSpace = true
				lx.prevWasDollar = false
				lx.wordState = wordNW
				continue
			}
			word := lx.readWordTail(r)
			if word == "" {
				return 0
			}
			node := W(word)
			node.Pos = Pos{Line: line, Col: col}
			lx.wordState = wordRW
			return lx.emitToken(WORD, node, lval)
		case ' ', '\t':
			lx.sawSpace = true
			if lx.prevWasDollar {
				lx.prevWasDollar = false
			}
			lx.wordState = wordNW
			continue
		case '#':
			if lx.prevWasDollar && !lx.sawSpace {
				return lx.emitToken(COUNT, nil, lval)
			}
			return lx.skipComment()
		case '\n':
			lx.prevConcat = false
			lx.prevWasDollar = false
			lx.sawSpace = false
			lx.wordState = wordNW
			return int(r)
		case '&':
			if lx.consumeIf('&') {
				return lx.emitToken(ANDAND, nil, lval)
			}
			lx.wordState = wordNW
			return lx.emitToken(int(r), nil, lval)
		case '|':
			if lx.consumeIf('|') {
				return lx.emitToken(OROR, nil, lval)
			}
			lx.readPair()
			left := lx.fdLeft
			right := lx.fdRight
			if left == fdUnset {
				left = 1
			}
			if right == fdUnset {
				right = 0
			}
			if right == fdClosed {
				lx.Error("expected digit after '='")
				return HUH
			}
			node := &Node{Kind: KPipe, I1: left, I2: right, Pos: Pos{Line: line, Col: col}}
			lx.wordState = wordNW
			return lx.emitToken(PIPE, node, lval)
		case '>':
			node, tok := lx.readRedir('>', line, col)
			return lx.emitToken(tok, node, lval)
		case '<':
			node, tok := lx.readRedir('<', line, col)
			return lx.emitToken(tok, node, lval)
		case '\'':
			text, ok := lx.readSingleQuoted()
			if !ok {
				lx.Error("unterminated quote")
				return 0
			}
			node := W(text)
			node.Pos = Pos{Line: line, Col: col}
			lx.wordState = wordRW
			return lx.emitToken(WORD, node, lval)
		case '(':
			if lx.wordState == wordRW && !lx.sawSpace {
				lx.wordState = wordNW
				return lx.emitToken(SUB, nil, lval)
			}
			lx.wordState = wordNW
			return lx.emitToken(int(r), nil, lval)
		case ')', '{', '}', ';', '^':
			lx.wordState = wordNW
			return lx.emitToken(int(r), nil, lval)
		case '=':
			lx.wordState = wordKW
			return lx.emitToken(int(r), nil, lval)
		case '@':
			lx.wordState = wordKW
			return lx.emitToken(SUBSHELL, nil, lval)
		case '!':
			lx.wordState = wordKW
			return lx.emitToken(BANG, nil, lval)
		case '~':
			lx.wordState = wordKW
			return lx.emitToken(TWIDDLE, nil, lval)
		case '$':
			next, _, _, err := lx.peekRune()
			if err == nil && next == '#' {
				_, _, _, _ = lx.readRune()
				return lx.emitToken(COUNT, nil, lval)
			}
			if err == nil && (next == '^' || next == '"') {
				_, _, _, _ = lx.readRune()
				return lx.emitToken(FLAT, nil, lval)
			}
			return lx.emitToken(int(r), nil, lval)
		case '`':
			if lx.consumeIf('`') {
				return lx.emitToken(BACKBACK, nil, lval)
			}
			return lx.emitToken(int(r), nil, lval)
		case '"':
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
				lx.wordState = wordKW
				return lx.emitToken(tok, node, lval)
			}
			lx.wordState = wordRW
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
	case "fn":
		return FN, true
	case "switch":
		return SWITCH, true
	case "else":
		return ELSE, true
	case "case":
		return CASE, true
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
		if r == '\\' {
			_, _, _, _ = lx.readRune()
			next, _, _, err := lx.peekRune()
			if err == nil && next == '\n' {
				_, _, _, _ = lx.readRune()
				lx.sawSpace = true
				break
			}
			b.WriteRune('\\')
			continue
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
		if r == '\\' {
			_, _, _, _ = lx.readRune()
			next, _, _, err := lx.peekRune()
			if err == nil && next == '\n' {
				_, _, _, _ = lx.readRune()
				lx.sawSpace = true
				break
			}
			b.WriteRune('\\')
			continue
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
	case ' ', '\t', '\n', ';', '&', '|', '(', ')', '{', '}', '=', '^', '$', '"', '\'', '`', '<', '>', '#', '@', '!', '~', '\\':
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
			next, _, _, err := lx.peekRune()
			if err == nil && next == '\'' {
				_, _, _, _ = lx.readRune()
				b.WriteRune('\'')
				continue
			}
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
	case WORD, int('`'), int('"'), BACKBACK:
		return true
	default:
		return false
	}
}

func canConcatTokenAfterDollar(tok int) bool {
	switch tok {
	case WORD, COUNT, FLAT, int('`'), int('"'), BACKBACK:
		return true
	default:
		return false
	}
}

func (lx *Lexer) skipComment() int {
	for {
		r, _, _, err := lx.readRune()
		if err != nil {
			lx.prevConcat = false
			lx.prevWasDollar = false
			lx.sawSpace = false
			return 0
		}
		if r == '\n' {
			lx.prevConcat = false
			lx.prevWasDollar = false
			lx.sawSpace = false
			return int('\n')
		}
	}
}

func (lx *Lexer) readRedir(op rune, line, col int) (*Node, int) {
	tok := REDIR
	rtype := ""
	if op == '>' {
		if lx.consumeIf('>') {
			rtype = ">>"
		} else {
			rtype = ">"
		}
	} else {
		if lx.consumeIf('<') {
			if lx.consumeIf('<') {
				rtype = "<<<"
			} else {
				rtype = "<<"
			}
		} else {
			rtype = "<"
		}
	}
	node := &Node{Kind: KRedir, Tok: rtype, I1: fdUnset, Pos: Pos{Line: line, Col: col}}
	if rtype == "<<" || rtype == "<<<" {
		tok = SREDIR
	}
	if lx.readPair() {
		if lx.fdRight == fdUnset {
			node.I1 = lx.fdLeft
			return node, tok
		}
		dup := &Node{Kind: KDup, Tok: rtype, I1: lx.fdLeft, I2: lx.fdRight, Pos: Pos{Line: line, Col: col}}
		return dup, DUP
	}
	return node, tok
}

func (lx *Lexer) readPair() bool {
	r, _, _, err := lx.peekRune()
	if err != nil || r != '[' {
		lx.fdLeft = fdUnset
		lx.fdRight = fdUnset
		return false
	}
	_, _, _, _ = lx.readRune()
	lx.fdLeft = fdUnset
	lx.fdRight = fdUnset
	n, ok := lx.readNumber()
	if !ok {
		lx.Error("expected digit after '['")
		return false
	}
	lx.fdLeft = n
	r, _, _, err = lx.readRune()
	if err != nil {
		return false
	}
	switch r {
	case ']':
		return true
	case '=':
		r, _, _, err = lx.peekRune()
		if err != nil {
			return false
		}
		if r == ']' {
			_, _, _, _ = lx.readRune()
			lx.fdRight = fdClosed
			return true
		}
		n, ok := lx.readNumber()
		if !ok {
			lx.Error("expected digit or ']' after '='")
			return false
		}
		lx.fdRight = n
		r, _, _, err = lx.readRune()
		if err != nil || r != ']' {
			lx.Error("expected ']' after digit")
			return false
		}
		return true
	default:
		lx.Error("expected '=' or ']' after digit")
		return false
	}
}

func (lx *Lexer) readNumber() (int, bool) {
	r, _, _, err := lx.readRune()
	if err != nil {
		return 0, false
	}
	if r < '0' || r > '9' {
		return 0, false
	}
	n := int(r - '0')
	for {
		r, _, _, err := lx.peekRune()
		if err != nil {
			break
		}
		if r < '0' || r > '9' {
			break
		}
		_, _, _, _ = lx.readRune()
		n = n*10 + int(r-'0')
	}
	return n, true
}
