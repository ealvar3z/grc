package parse

// PreorderWords collects Tok values for KWord nodes in preorder.
func PreorderWords(n *Node) []string {
	if n == nil {
		return nil
	}
	var out []string
	if n.Kind == KWord && n.Tok != "" {
		out = append(out, n.Tok)
	}
	out = append(out, PreorderWords(n.Left)...)
	out = append(out, PreorderWords(n.Right)...)
	for _, child := range n.List {
		out = append(out, PreorderWords(child)...)
	}
	return out
}

// KindsPreorder collects node kinds in preorder.
func KindsPreorder(n *Node) []Kind {
	if n == nil {
		return nil
	}
	out := []Kind{n.Kind}
	out = append(out, KindsPreorder(n.Left)...)
	out = append(out, KindsPreorder(n.Right)...)
	for _, child := range n.List {
		out = append(out, KindsPreorder(child)...)
	}
	return out
}

// FindFirstKind returns the first node with the given kind in preorder.
func FindFirstKind(n *Node, k Kind) *Node {
	if n == nil {
		return nil
	}
	if n.Kind == k {
		return n
	}
	if found := FindFirstKind(n.Left, k); found != nil {
		return found
	}
	if found := FindFirstKind(n.Right, k); found != nil {
		return found
	}
	for _, child := range n.List {
		if found := FindFirstKind(child, k); found != nil {
			return found
		}
	}
	return nil
}
