package eval

import (
	"fmt"

	"grc/internal/parse"
)

// RedirPlan captures a redirection operator and its target words.
type RedirPlan struct {
	Op     string
	Target []string
	Fd     int
	DupTo  int
	Close  bool
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
	IfCond     *parse.Node
	IfBody     *parse.Node
	IfElse     *parse.Node
	ForName    string
	ForList    *parse.Node
	ForBody    *parse.Node
	WhileCond  *parse.Node
	WhileBody  *parse.Node
	SwitchArg  *parse.Node
	SwitchBody *parse.Node
	NotBody    *parse.Node
	SubBody    *parse.Node
	MatchSubj  *parse.Node
	MatchPats  *parse.Node
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
	PlanIf
	PlanFor
	PlanWhile
	PlanSwitch
	PlanNot
	PlanSubshell
	PlanTwiddle
	PlanFnRm
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
		if ast.Right == nil {
			return BuildPlan(ast.Left, env)
		}
		plan, err := BuildPlan(ast.Left, env)
		if err != nil {
			return nil, err
		}
		if err := applyRedirsFromNode(plan, ast.Right, env); err != nil {
			return nil, err
		}
		return plan, nil
	case parse.KParen:
		return BuildPlan(ast.Left, env)
	case parse.KIf:
		if ast.Right != nil && ast.Right.Kind == parse.KElse {
			return &ExecPlan{Kind: PlanIf, IfCond: ast.Left, IfBody: ast.Right.Left, IfElse: ast.Right.Right}, nil
		}
		return &ExecPlan{Kind: PlanIf, IfCond: ast.Left, IfBody: ast.Right}, nil
	case parse.KFor:
		name := fnName(ast.Left)
		var list *parse.Node
		if len(ast.List) > 0 {
			list = &parse.Node{Kind: parse.KWords, List: ast.List}
		}
		return &ExecPlan{Kind: PlanFor, ForName: name, ForList: list, ForBody: ast.Right}, nil
	case parse.KWhile:
		return &ExecPlan{Kind: PlanWhile, WhileCond: ast.Left, WhileBody: ast.Right}, nil
	case parse.KSwitch:
		return &ExecPlan{Kind: PlanSwitch, SwitchArg: ast.Left, SwitchBody: ast.Right}, nil
	case parse.KBang:
		return &ExecPlan{Kind: PlanNot, NotBody: ast.Left}, nil
	case parse.KSubshell:
		return &ExecPlan{Kind: PlanSubshell, SubBody: ast.Left}, nil
	case parse.KMatch:
		return &ExecPlan{Kind: PlanTwiddle, MatchSubj: ast.Left, MatchPats: ast.Right}, nil
	case parse.KFnRm:
		name := fnName(ast.Left)
		if name == "" {
			return &ExecPlan{Kind: PlanNoop}, nil
		}
		def := &FuncDef{Name: name}
		return &ExecPlan{Kind: PlanFnRm, Func: def}, nil
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
		plan.Redirs = append(plan.Redirs, RedirPlan{Op: ast.Tok, Target: target, Fd: ast.I1})
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
	case parse.KPre:
		return buildPlanPre(ast, env)
	default:
		return nil, fmt.Errorf("unsupported AST node: %v", ast.Kind)
	}
}

func buildPlanPre(ast *parse.Node, env *Env) (*ExecPlan, error) {
	prefixes, redirs, rest := splitPre(ast)
	if rest == nil {
		if len(prefixes) > 0 {
			var head *ExecPlan
			var tail *ExecPlan
			for _, pref := range prefixes {
				node := &ExecPlan{Kind: PlanAssign, AssignName: pref.Name, AssignVal: pref.Val}
				if head == nil {
					head = node
					tail = node
				} else {
					tail.Next = node
					tail = node
				}
			}
			plan := head
			for _, r := range redirs {
				if err := applyRedirsFromNode(plan, r, env); err != nil {
					return nil, err
				}
			}
			return plan, nil
		}
		plan := &ExecPlan{Kind: PlanNoop}
		for _, r := range redirs {
			if err := applyRedirsFromNode(plan, r, env); err != nil {
				return nil, err
			}
		}
		return plan, nil
	}
	plan, err := BuildPlan(rest, env)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return plan, nil
	}
	if len(prefixes) > 0 {
		plan.Prefix = append(prefixes, plan.Prefix...)
	}
	for _, r := range redirs {
		if err := applyRedirsFromNode(plan, r, env); err != nil {
			return nil, err
		}
	}
	return plan, nil
}

func applyRedirsFromNode(plan *ExecPlan, n *parse.Node, env *Env) error {
	if n == nil || plan == nil {
		return nil
	}
	if n.Kind == parse.KEpilog {
		for _, child := range n.List {
			if err := applyRedirsFromNode(plan, child, env); err != nil {
				return err
			}
		}
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
			plan.Redirs = append(plan.Redirs, RedirPlan{Op: child.Tok, Target: target, Fd: child.I1})
		}
		return nil
	}
	if n.Kind == parse.KRedir {
		target, err := ExpandWord(n.Right, env)
		if err != nil {
			return err
		}
		plan.Redirs = append(plan.Redirs, RedirPlan{Op: n.Tok, Target: target, Fd: n.I1})
		return nil
	}
	if n.Kind == parse.KDup {
		plan.Redirs = append(plan.Redirs, RedirPlan{Op: "dup", Fd: n.I1, DupTo: n.I2, Close: n.I2 < 0})
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
	if n.Left != nil {
		name := fnName(n.Left)
		return name, n.Right, false
	}
	return "", nil, false
}

func splitPrefixAssign(n *parse.Node) ([]AssignPrefix, *parse.Node) {
	var prefixes []AssignPrefix
	cur := n
	for cur != nil && cur.Kind == parse.KAssign && cur.Right != nil {
		name := fnName(cur.Left)
		if name == "" {
			break
		}
		prefixes = append(prefixes, AssignPrefix{Name: name, Val: cur.Right})
		break
	}
	return prefixes, nil
}

func splitPre(n *parse.Node) ([]AssignPrefix, []*parse.Node, *parse.Node) {
	var prefixes []AssignPrefix
	var redirs []*parse.Node
	cur := n
	for cur != nil && cur.Kind == parse.KPre {
		if cur.Left == nil {
			break
		}
		switch cur.Left.Kind {
		case parse.KAssign:
			name := fnName(cur.Left.Left)
			if name != "" {
				prefixes = append(prefixes, AssignPrefix{Name: name, Val: cur.Left.Right})
			}
		case parse.KRedir, parse.KDup:
			redirs = append(redirs, cur.Left)
		}
		cur = cur.Right
	}
	return prefixes, redirs, cur
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
