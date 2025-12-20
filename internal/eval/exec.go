package eval

import (
	"fmt"

	"grc/internal/parse"
)

// RedirPlan captures a redirection operator and its target words.
type RedirPlan struct {
	Op     string
	Target []string
}

// ExecPlan is a dry-run execution plan.
type ExecPlan struct {
	Argv       []string
	Redirs     []RedirPlan
	PipeTo     *ExecPlan
	Next       *ExecPlan
	IfOK       *ExecPlan
	IfFail     *ExecPlan
	Background bool
}

// BuildPlan converts an AST into an execution plan.
func BuildPlan(ast *parse.Node, env *Env) (*ExecPlan, error) {
	if ast == nil {
		return nil, nil
	}
	switch ast.Kind {
	case parse.KSeq:
		left, err := BuildPlan(ast.Left, env)
		if err != nil {
			return nil, err
		}
		right, err := BuildPlan(ast.Right, env)
		if err != nil {
			return nil, err
		}
		if left == nil {
			return right, nil
		}
		tail := Tail(left)
		tail.Next = right
		return left, nil
	case parse.KPipe:
		left, err := BuildPlan(ast.Left, env)
		if err != nil {
			return nil, err
		}
		right, err := BuildPlan(ast.Right, env)
		if err != nil {
			return nil, err
		}
		if left == nil {
			return right, nil
		}
		left.PipeTo = right
		return left, nil
	case parse.KBg:
		plan, err := BuildPlan(ast.Left, env)
		if err != nil {
			return nil, err
		}
		if plan != nil {
			plan.Background = true
		}
		return plan, nil
	case parse.KAnd:
		left, err := BuildPlan(ast.Left, env)
		if err != nil {
			return nil, err
		}
		right, err := BuildPlan(ast.Right, env)
		if err != nil {
			return nil, err
		}
		if left == nil {
			return right, nil
		}
		Tail(left).IfOK = right
		return left, nil
	case parse.KOr:
		left, err := BuildPlan(ast.Left, env)
		if err != nil {
			return nil, err
		}
		right, err := BuildPlan(ast.Right, env)
		if err != nil {
			return nil, err
		}
		if left == nil {
			return right, nil
		}
		Tail(left).IfFail = right
		return left, nil
	case parse.KRedir:
		plan, err := BuildPlan(ast.Left, env)
		if err != nil {
			return nil, err
		}
		target, err := ExpandWord(ast.Right, env)
		if err != nil {
			return nil, err
		}
		if plan == nil {
			plan = &ExecPlan{}
		}
		plan.Redirs = append(plan.Redirs, RedirPlan{Op: ast.Tok, Target: target})
		return plan, nil
	case parse.KCall:
		argv, err := ExpandCall(ast, env)
		if err != nil {
			return nil, err
		}
		plan := &ExecPlan{Argv: argv}
		if err := applyRedirsFromNode(plan, ast.Right, env); err != nil {
			return nil, err
		}
		return plan, nil
	default:
		return nil, fmt.Errorf("unsupported AST node: %v", ast.Kind)
	}
}

func applyRedirsFromNode(plan *ExecPlan, n *parse.Node, env *Env) error {
	if n == nil || plan == nil {
		return nil
	}
	if n.Kind == parse.KRedir && len(n.List) > 0 {
		for _, child := range n.List {
			if child == nil {
				continue
			}
			target, err := ExpandWord(child.Right, env)
			if err != nil {
				return err
			}
			plan.Redirs = append(plan.Redirs, RedirPlan{Op: child.Tok, Target: target})
		}
		return nil
	}
	if n.Kind == parse.KRedir {
		target, err := ExpandWord(n.Right, env)
		if err != nil {
			return err
		}
		plan.Redirs = append(plan.Redirs, RedirPlan{Op: n.Tok, Target: target})
		return nil
	}
	return nil
}

// Tail returns the last plan in the Next chain.
func Tail(p *ExecPlan) *ExecPlan {
	if p == nil {
		return nil
	}
	cur := p
	for cur.Next != nil {
		cur = cur.Next
	}
	return cur
}
