package eval

import (
	"fmt"

	"grc/internal/parse"
)

// ExpandWord expands a word node into a list of strings.
func ExpandWord(n *parse.Node, env *Env) ([]string, error) {
	if n == nil {
		return nil, nil
	}
	switch n.Kind {
	case parse.KWord:
		return []string{n.Tok}, nil
	case parse.KConcat:
		left, err := ExpandWord(n.Left, env)
		if err != nil {
			return nil, err
		}
		right, err := ExpandWord(n.Right, env)
		if err != nil {
			return nil, err
		}
		out := concatProduct(left, right)
		if out == nil {
			return nil, fmt.Errorf("concat length mismatch")
		}
		return out, nil
	case parse.KDollar:
		if n.Left == nil || n.Left.Kind != parse.KWord {
			return nil, fmt.Errorf("unsupported dollar node")
		}
		vals := env.Get(n.Left.Tok)
		if vals == nil {
			return []string{}, nil
		}
		return vals, nil
	default:
		return nil, fmt.Errorf("unsupported word node: %v", n.Kind)
	}
}

// ExpandCall flattens a call node into an argv list.
func ExpandCall(n *parse.Node, env *Env) ([]string, error) {
	if n == nil {
		return nil, nil
	}
	if n.Kind != parse.KCall {
		return nil, fmt.Errorf("expected call node, got %v", n.Kind)
	}
	return expandArgs(n.Left, env)
}

func expandArgs(n *parse.Node, env *Env) ([]string, error) {
	if n == nil {
		return nil, nil
	}
	if n.Kind == parse.KArgList || n.Kind == parse.KWords {
		var out []string
		for _, child := range n.List {
			vals, err := expandArgs(child, env)
			if err != nil {
				return nil, err
			}
			out = append(out, vals...)
		}
		return out, nil
	}
	vals, err := ExpandWord(n, env)
	if err != nil {
		return nil, err
	}
	return vals, nil
}

func concatProduct(left, right []string) []string {
	if len(left) == 0 || len(right) == 0 {
		return []string{}
	}
	if len(left) == len(right) {
		out := make([]string, 0, len(left))
		for i := range left {
			out = append(out, left[i]+right[i])
		}
		return out
	}
	if len(left) == 1 {
		out := make([]string, 0, len(right))
		for _, r := range right {
			out = append(out, left[0]+r)
		}
		return out
	}
	if len(right) == 1 {
		out := make([]string, 0, len(left))
		for _, l := range left {
			out = append(out, l+right[0])
		}
		return out
	}
	return nil
}
