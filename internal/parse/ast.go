package parse

// Kind represents the AST node kind.
type Kind int

const (
	KSeq Kind = iota
	KBg
	KPipe
	KAnd
	KOr
	KAssign
	KRedir
	KDup
	KPre
	KEpilog
	KCall
	KWord
	KWords
	KArgList
	KBrace
	KParen
	KSubshell
	KFnDef
	KFnRm
	KFn
	KSwitch
	KCase
	KCbody
	KIf
	KElse
	KWhile
	KFor
	KMatch
	KVar
	KSub
	KQuote
	KDollar
	KCount
	KFlat
	KBackquote
	KConcat
	KBang
	KArgs
	KLappend
	KNmpipe
)

// Pos tracks a source position.
type Pos struct {
	Line int
	Col  int
}

// Node is a minimal AST node.
type Node struct {
	Kind        Kind
	Tok         string
	Pos         Pos
	Left, Right *Node
	List        []*Node
	I1          int
	I2          int
}

// N constructs a binary node.
func N(k Kind, a, b *Node) *Node {
	return &Node{Kind: k, Left: a, Right: b}
}

// W constructs a word node.
func W(s string) *Node {
	return &Node{Kind: KWord, Tok: s}
}

// L constructs or appends to a list node.
func L(k Kind, xs ...*Node) *Node {
	var out *Node
	for _, n := range xs {
		if n == nil {
			continue
		}
		if out == nil {
			if n.Kind == k && len(n.List) > 0 {
				out = n
				continue
			}
			out = &Node{Kind: k}
		}
		if n.Kind == k && len(n.List) > 0 {
			out.List = append(out.List, n.List...)
			continue
		}
		out.List = append(out.List, n)
	}
	if out == nil {
		out = &Node{Kind: k}
	}
	return out
}

// File is a minimal AST root placeholder until the real grammar is wired in.
type File struct {
	Text string
}
