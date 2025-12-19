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
}

func NewLexer(rd io.Reader) *Lexer {
	return &Lexer{r: bufio.NewReader(rd)}
}

func (lx *Lexer) Lex(lval *grcSymType) int {
	for {
		ch, _, err := lx.r.ReadRune()
		if err != nil {
			return 0
		}

		switch ch {
		case ' ', '\t':
			continue
		case '\n':
			return int(ch)
		case '&':
			next, _, err := lx.r.ReadRune()
			if err == nil {
				if next == '&' {
					return ANDAND
				}
				_ = lx.r.UnreadRune()
			}
			return int(ch)
		case '|':
			next, _, err := lx.r.ReadRune()
			if err == nil {
				if next == '|' {
					return OROR
				}
				_ = lx.r.UnreadRune()
			}
			return PIPE
		case '>':
			next, _, err := lx.r.ReadRune()
			if err == nil {
				if next == '>' {
					lval.node = &Node{Kind: KRedir, Tok: ">>"}
					return REDIR
				}
				_ = lx.r.UnreadRune()
			}
			lval.node = &Node{Kind: KRedir, Tok: ">"}
			return REDIR
		case '<':
			lval.node = &Node{Kind: KRedir, Tok: "<"}
			return REDIR
		case ';', '(', ')', '{', '}', '=', '^', '$', '"', '`':
			return int(ch)
		default:
			lx.r.UnreadRune()
			word := lx.readWord()
			if word == "" {
				return 0
			}
			if tok, ok := keywordToken(word); ok {
				lval.node = W(word)
				return tok
			}
			lval.node = W(word)
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

func (lx *Lexer) readWord() string {
	var b strings.Builder
	for {
		ch, _, err := lx.r.ReadRune()
		if err != nil {
			break
		}
		if isWordBreak(ch) {
			_ = lx.r.UnreadRune()
			break
		}
		b.WriteRune(ch)
	}
	return b.String()
}

func isWordBreak(ch rune) bool {
	switch ch {
	case ' ', '\t', '\n', ';', '&', '|', '(', ')', '{', '}', '=', '^', '$', '"', '`', '<', '>':
		return true
	default:
		return false
	}
}
