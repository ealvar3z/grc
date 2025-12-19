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
	KCall
	KWord
	KWords
	KArgList
	KBrace
	KParen
	KSubshell
	KFnDef
	KFn
	KSwitch
	KSub
	KQuote
	KDollar
	KCount
	KBackquote
)

// Node is a minimal AST node.
type Node struct {
	Kind        Kind
	Tok         string
	Left, Right *Node
	List        []*Node
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
	n := &Node{Kind: k}
	n.List = append(n.List, xs...)
	return n
}

// File is a minimal AST root placeholder until the real grammar is wired in.
type File struct {
	Text string
}
