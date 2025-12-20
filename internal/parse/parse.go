package parse

import (
	"fmt"
	"io"
)

// Parse reads input and returns the parsed AST.
func Parse(rd io.Reader) (*Node, error) {
	lx := NewLexer(rd)
	return ParseWithLexer(lx)
}

// ParseWithLexer parses a single form from the lexer.
func ParseWithLexer(lx *Lexer) (*Node, error) {
	parseResult = nil
	if grcParse(lx) != 0 && parseResult == nil {
		if lx.Err != nil {
			return nil, lx.Err
		}
		return nil, fmt.Errorf("parse error")
	}
	if lx.Err != nil {
		return nil, lx.Err
	}
	if parseResult == nil {
		return nil, fmt.Errorf("parse error")
	}
	return parseResult, nil
}

// ParseAll reads all forms and returns a sequence AST.
func ParseAll(rd io.Reader) (*Node, error) {
	lx := NewLexer(rd)
	var prog *Node
	for {
		parseResult = nil
		if grcParse(lx) != 0 {
			if lx.Err != nil {
				return nil, lx.Err
			}
			if parseResult == nil {
				if lx.EOF() {
					break
				}
				continue
			}
		}
		if lx.Err != nil {
			return nil, lx.Err
		}
		if parseResult == nil {
			continue
		}
		prog = appendSeq(prog, parseResult)
	}
	return prog, nil
}

func appendSeq(prog, next *Node) *Node {
	if next == nil {
		return prog
	}
	if prog == nil {
		return next
	}
	if prog.Kind == KSeq {
		prog.Right = appendSeq(prog.Right, next)
		return prog
	}
	return N(KSeq, prog, next)
}
