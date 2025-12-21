package eval

import (
	"path/filepath"
	"strings"

	"grc/internal/parse"
)

type caseBlock struct {
	Patterns []string
	Body     *parse.Node
}

func switchCases(n *parse.Node, env *Env) ([]caseBlock, error) {
	if n == nil {
		return nil, nil
	}
	body := n
	if n.Kind == parse.KBrace {
		body = n.Left
	}
	cmds := flattenSeq(body)
	var out []caseBlock
	var cur *caseBlock
	for _, cmd := range cmds {
		patterns, ok, err := casePatterns(cmd, env)
		if err != nil {
			return nil, err
		}
		if ok {
			if cur != nil {
				out = append(out, *cur)
			}
			cur = &caseBlock{Patterns: patterns}
			continue
		}
		if cur != nil {
			cur.Body = appendSeq(cur.Body, cmd)
		}
	}
	if cur != nil {
		out = append(out, *cur)
	}
	return out, nil
}

func casePatterns(cmd *parse.Node, env *Env) ([]string, bool, error) {
	call := unwrapCall(cmd)
	if call == nil {
		return nil, false, nil
	}
	args, err := ExpandWordsNoGlob(call.Left, env)
	if err != nil || len(args) == 0 {
		return nil, false, err
	}
	if args[0] != "case" {
		return nil, false, nil
	}
	return args[1:], true, nil
}

func unwrapCall(n *parse.Node) *parse.Node {
	for n != nil && n.Kind == parse.KRedir {
		n = n.Left
	}
	if n != nil && n.Kind == parse.KCall {
		return n
	}
	return nil
}

func flattenSeq(n *parse.Node) []*parse.Node {
	if n == nil {
		return nil
	}
	if n.Kind == parse.KSeq {
		out := flattenSeq(n.Left)
		out = append(out, flattenSeq(n.Right)...)
		return out
	}
	return []*parse.Node{n}
}

func appendSeq(left, right *parse.Node) *parse.Node {
	if left == nil {
		return right
	}
	return &parse.Node{Kind: parse.KSeq, Left: left, Right: right}
}

func matchAnyPattern(arg string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}
	for _, pat := range patterns {
		if rcMatch(pat, arg) {
			return true
		}
	}
	return false
}

func rcMatch(pattern, subject string) bool {
	if !dotMatchAllowed(pattern, subject) {
		return false
	}
	ok, err := filepath.Match(pattern, subject)
	return err == nil && ok
}

func dotMatchAllowed(pattern, subject string) bool {
	if strings.HasPrefix(subject, ".") && !strings.HasPrefix(pattern, ".") {
		return false
	}
	if strings.Contains(subject, "/.") && !strings.Contains(pattern, "/.") {
		return false
	}
	psegs := strings.Split(pattern, "/")
	ssegs := strings.Split(subject, "/")
	if len(psegs) != len(ssegs) {
		return true
	}
	for i, seg := range ssegs {
		if strings.HasPrefix(seg, ".") && !strings.HasPrefix(psegs[i], ".") {
			return false
		}
	}
	return true
}
