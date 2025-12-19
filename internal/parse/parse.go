package parse

import (
	"fmt"
	"io"
)

// Parse reads input and returns the parsed AST.
func Parse(rd io.Reader) (*Node, error) {
	lx := NewLexer(rd)
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
