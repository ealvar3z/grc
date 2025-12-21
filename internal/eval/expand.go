package eval

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"grc/internal/parse"
)

// ExpandWord expands a word node into a list of strings.
func ExpandWord(n *parse.Node, env *Env) ([]string, error) {
	if n == nil {
		return nil, nil
	}
	words, err := expandWordBase(n, env)
	if err != nil {
		return nil, err
	}
	return globWords(words)
}

// ExpandWordNoGlob expands a word without globbing.
func ExpandWordNoGlob(n *parse.Node, env *Env) ([]string, error) {
	if n == nil {
		return nil, nil
	}
	return expandWordBase(n, env)
}

func expandWordBase(n *parse.Node, env *Env) ([]string, error) {
	if n == nil {
		return nil, nil
	}
	switch n.Kind {
	case parse.KWord:
		return []string{n.Tok}, nil
	case parse.KConcat:
		left, err := expandWordBase(n.Left, env)
		if err != nil {
			return nil, err
		}
		right, err := expandWordBase(n.Right, env)
		if err != nil {
			return nil, err
		}
		out := concatProduct(left, right)
		if out == nil {
			return nil, fmt.Errorf("concat length mismatch")
		}
		return out, nil
	case parse.KVar:
		if n.Left == nil || n.Left.Kind != parse.KWord {
			return nil, fmt.Errorf("unsupported var node")
		}
		vals := env.Get(n.Left.Tok)
		if vals == nil {
			return []string{}, nil
		}
		if n.Right != nil {
			subs, err := expandArgsNoGlob(n.Right, env)
			if err != nil {
				return nil, err
			}
			return applySubscript(vals, subs), nil
		}
		return vals, nil
	case parse.KFlat:
		if n.Left == nil || n.Left.Kind != parse.KWord {
			return nil, fmt.Errorf("unsupported flat node")
		}
		vals := env.Get(n.Left.Tok)
		if vals == nil || len(vals) == 0 {
			return []string{""}, nil
		}
		return []string{strings.Join(vals, " ")}, nil
	case parse.KCount:
		if n.Left == nil || n.Left.Kind != parse.KWord {
			return nil, fmt.Errorf("unsupported count node")
		}
		vals := env.Get(n.Left.Tok)
		return []string{fmt.Sprintf("%d", len(vals))}, nil
	case parse.KSub:
		vals, err := expandWordBase(n.Left, env)
		if err != nil {
			return nil, err
		}
		if len(vals) == 0 {
			return []string{}, nil
		}
		subs, err := expandArgsNoGlob(n.Right, env)
		if err != nil {
			return nil, err
		}
		return applySubscript(vals, subs), nil
	case parse.KBackquote:
		if env == nil {
			env = NewEnv(nil)
		}
		child := NewChild(env)
		plan, err := BuildPlan(n.Right, child)
		if err != nil {
			return nil, err
		}
		var out bytes.Buffer
		runner := &Runner{Env: child}
		res := runner.RunPlan(plan, strings.NewReader(""), &out, io.Discard)
		if env != nil {
			env.SetStatus(res.Status)
		}
		fields := splitFields(out.String(), env, n.Left)
		if len(fields) == 0 {
			return []string{}, nil
		}
		return fields, nil
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

// ExpandValue expands a value node for assignments.
func ExpandValue(n *parse.Node, env *Env) ([]string, error) {
	if n == nil {
		return []string{}, nil
	}
	switch n.Kind {
	case parse.KParen:
		return normalizeEmpty(expandArgs(n.Left, env))
	case parse.KWords, parse.KArgList:
		return normalizeEmpty(expandArgs(n, env))
	default:
		vals, err := ExpandWord(n, env)
		if err != nil {
			return nil, err
		}
		if vals == nil {
			return []string{}, nil
		}
		return vals, nil
	}
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

// ExpandWordsNoGlob expands a list without globbing.
func ExpandWordsNoGlob(n *parse.Node, env *Env) ([]string, error) {
	return expandArgsNoGlob(n, env)
}

func expandArgsNoGlob(n *parse.Node, env *Env) ([]string, error) {
	if n == nil {
		return nil, nil
	}
	if n.Kind == parse.KArgList || n.Kind == parse.KWords {
		var out []string
		for _, child := range n.List {
			vals, err := expandArgsNoGlob(child, env)
			if err != nil {
				return nil, err
			}
			out = append(out, vals...)
		}
		return out, nil
	}
	return ExpandWordNoGlob(n, env)
}

func normalizeEmpty(vals []string, err error) ([]string, error) {
	if err != nil {
		return nil, err
	}
	if vals == nil {
		return []string{}, nil
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

func globWords(words []string) ([]string, error) {
	var out []string
	for _, w := range words {
		matches, err := GlobWord(w)
		if err != nil {
			return nil, err
		}
		out = append(out, matches...)
	}
	return out, nil
}

// GlobWord expands glob patterns in w.
func GlobWord(w string) ([]string, error) {
	if !strings.ContainsAny(w, "*?[") {
		return []string{w}, nil
	}
	matches, err := filepath.Glob(w)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return []string{w}, nil
	}
	sort.Strings(matches)
	return matches, nil
}

func splitFields(s string, env *Env, ifsOverride *parse.Node) []string {
	if s == "" {
		return []string{}
	}
	ifs := []rune{' ', '\t', '\n'}
	if env != nil {
		if ifsOverride != nil {
			vals, err := ExpandWordsNoGlob(ifsOverride, env)
			if err == nil && len(vals) > 0 {
				ifs = []rune(strings.Join(vals, ""))
			}
		} else if vals := env.Get("ifs"); len(vals) > 0 {
			ifs = []rune(strings.Join(vals, ""))
		}
	}
	isSep := make(map[rune]bool, len(ifs))
	for _, r := range ifs {
		isSep[r] = true
	}
	f := func(r rune) bool {
		return isSep[r]
	}
	return strings.FieldsFunc(s, f)
}

func applySubscript(vals, subs []string) []string {
	if len(vals) == 0 || len(subs) == 0 {
		return []string{}
	}
	var out []string
	for _, sub := range subs {
		start, end, ok := parseRange(sub)
		if !ok {
			continue
		}
		if end == 0 {
			end = len(vals)
		}
		if start < 1 {
			continue
		}
		if start > len(vals) {
			continue
		}
		if end > len(vals) {
			end = len(vals)
		}
		for i := start - 1; i < end; i++ {
			out = append(out, vals[i])
		}
	}
	return out
}

func parseRange(s string) (int, int, bool) {
	if s == "" {
		return 0, 0, false
	}
	if strings.Contains(s, "-") {
		parts := strings.SplitN(s, "-", 2)
		start, ok := parsePositive(parts[0])
		if !ok {
			return 0, 0, false
		}
		if parts[1] == "" {
			return start, 0, true
		}
		end, ok := parsePositive(parts[1])
		if !ok {
			return 0, 0, false
		}
		if end < start {
			return 0, 0, false
		}
		return start, end, true
	}
	start, ok := parsePositive(s)
	if !ok {
		return 0, 0, false
	}
	return start, start, true
}

func parsePositive(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
}
