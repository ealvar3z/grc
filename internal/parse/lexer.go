package parse

// Token is a small placeholder for future lexer output.
type Token struct {
	Kind string
	Lexeme string
}

// Lexer is a stub implementation until the grammar is generated.
type Lexer struct {
	Tokens []Token
}

func (l *Lexer) Next() (Token, bool) {
	if len(l.Tokens) == 0 {
		return Token{}, false
	}
	ok := l.Tokens[0]
	l.Tokens = l.Tokens[1:]
	return ok, true
}
