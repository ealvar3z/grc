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

// concatNode builds a concat node, promoting a bare word to KDollar when
// concatenating with a dollar expansion to match rc-style $x^y expectations.
func concatNode(a, b *Node) *Node {
	if a != nil && a.Kind == KDollar && b != nil && b.Kind == KWord {
		b = N(KDollar, b, nil)
	}
	return N(KConcat, a, b)
}
