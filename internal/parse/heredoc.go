package parse

// ParseHereDocContent builds a concat chain for heredoc content with $ expansion.
func ParseHereDocContent(s string) *Node {
	var result *Node
	for i := 0; i < len(s); {
		switch s[i] {
		case '$':
			if i+1 < len(s) && s[i+1] == '$' {
				result = concatNode(result, W("$"))
				i += 2
				continue
			}
			i++
			start := i
			for i < len(s) && isVarChar(s[i]) {
				i++
			}
			if start == i {
				result = concatNode(result, W("$"))
				continue
			}
			if i < len(s) && s[i] == '^' {
				i++
			}
			name := s[start:i]
			result = concatNode(result, N(KFlat, W(name), nil))
		default:
			start := i
			for i < len(s) && s[i] != '$' {
				i++
			}
			if start < i {
				result = concatNode(result, W(s[start:i]))
			}
		}
	}
	return result
}

func concatNode(a, b *Node) *Node {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	return N(KConcat, a, b)
}

func isVarChar(b byte) bool {
	if b >= 'a' && b <= 'z' {
		return true
	}
	if b >= 'A' && b <= 'Z' {
		return true
	}
	if b >= '0' && b <= '9' {
		return true
	}
	return b == '_' || b == '*'
}
