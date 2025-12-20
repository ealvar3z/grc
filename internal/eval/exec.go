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
	Kind       PlanKind
	Argv       []string
	Prefix     []AssignPrefix
	Call       *parse.Node
	Redirs     []RedirPlan
	PipeTo     *ExecPlan
	Next       *ExecPlan
	IfOK       *ExecPlan
	IfFail     *ExecPlan
	Background bool
	Func       *FuncDef
	AssignName string
	AssignVal  *parse.Node
}

// AssignPrefix holds a temporary assignment for a command invocation.
type AssignPrefix struct {
	Name string
	Val  *parse.Node
}

// PlanKind describes the plan node type.
type PlanKind int

const (
	PlanCmd PlanKind = iota
	PlanFnDef
	PlanNoop
	PlanAssign
)

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
	case parse.KBrace:
		return BuildPlan(ast.Left, env)
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
		plan := &ExecPlan{Kind: PlanCmd, Argv: argv, Call: ast}
		if err := applyRedirsFromNode(plan, ast.Right, env); err != nil {
			return nil, err
		}
		return plan, nil
	case parse.KFnDef:
		name := fnName(ast.Left)
		if name == "" {
			return &ExecPlan{Kind: PlanNoop}, nil
		}
		def := &FuncDef{Name: name, Body: ast.Right}
		return &ExecPlan{Kind: PlanFnDef, Func: def}, nil
	case parse.KAssign:
		prefixes, rest := splitPrefixAssign(ast)
		if len(prefixes) > 0 && rest != nil {
			plan, err := BuildPlan(rest, env)
			if err != nil {
				return nil, err
			}
			if plan == nil {
				return plan, nil
			}
			if plan.Kind != PlanCmd {
				return nil, fmt.Errorf("assignment prefixes require a command")
			}
			plan.Prefix = append(prefixes, plan.Prefix...)
			return plan, nil
		}
		name, val, hasCmd := assignParts(ast)
		if name == "" {
			return &ExecPlan{Kind: PlanNoop}, nil
		}
		if hasCmd {
			return nil, fmt.Errorf("assignment prefixes not supported")
		}
		return &ExecPlan{Kind: PlanAssign, AssignName: name, AssignVal: val}, nil
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

func fnName(n *parse.Node) string {
	if n == nil {
		return ""
	}
	if n.Kind == parse.KWord && n.Tok != "" {
		return n.Tok
	}
	for _, child := range n.List {
		if name := fnName(child); name != "" {
			return name
		}
	}
	if name := fnName(n.Left); name != "" {
		return name
	}
	if name := fnName(n.Right); name != "" {
		return name
	}
	return ""
}

// assignParts extracts name/value from assignment nodes.
// Shape: KAssign( KAssign(name, value), cmd ) for standalone or prefix.
func assignParts(n *parse.Node) (string, *parse.Node, bool) {
	if n == nil || n.Kind != parse.KAssign {
		return "", nil, false
	}
	if n.Left != nil && n.Left.Kind == parse.KAssign && n.Left.Left != nil {
		name := fnName(n.Left.Left)
		return name, n.Left.Right, n.Right != nil
	}
	if n.Left != nil {
		name := fnName(n.Left)
		return name, n.Right, false
	}
	return "", nil, false
}

func splitPrefixAssign(n *parse.Node) ([]AssignPrefix, *parse.Node) {
	var prefixes []AssignPrefix
	cur := n
	for cur != nil && cur.Kind == parse.KAssign && cur.Left != nil && cur.Left.Kind == parse.KAssign && cur.Right != nil {
		name := fnName(cur.Left.Left)
		if name == "" {
			break
		}
		prefixes = append(prefixes, AssignPrefix{Name: name, Val: cur.Left.Right})
		cur = cur.Right
	}
	return prefixes, cur
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
