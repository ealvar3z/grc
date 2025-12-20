package parse

// buildCallFromSimple constructs a call node and wraps any redirections.
func buildCallFromSimple(n *Node) *Node {
	if n == nil {
		return &Node{Kind: KCall}
	}
	var args []*Node
	var redirs []*Node
	if n.Kind == KArgList {
		for _, child := range n.List {
			if child == nil {
				continue
			}
			if child.Kind == KRedir {
				redirs = append(redirs, child)
				continue
			}
			args = append(args, child)
		}
	} else if n.Kind == KRedir {
		redirs = append(redirs, n)
	} else {
		args = append(args, n)
	}
	var argNode *Node
	if len(args) > 0 {
		argNode = L(KArgList, args...)
	}
	call := N(KCall, argNode, nil)
	for _, r := range redirs {
		r.Left = call
		call = r
	}
	return call
}
