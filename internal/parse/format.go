package parse

import (
	"fmt"
	"strings"
)

// Format renders an AST subtree as rc syntax (best-effort).
func Format(n *Node) (string, error) {
	if n == nil {
		return "", nil
	}
	return formatNode(n), nil
}

func formatNode(n *Node) string {
	if n == nil {
		return ""
	}
	switch n.Kind {
	case KSeq:
		return joinNonEmpty([]string{formatNode(n.Left), formatNode(n.Right)}, "; ")
	case KPipe:
		return joinNonEmpty([]string{formatNode(n.Left), formatNode(n.Right)}, " | ")
	case KAnd:
		return joinNonEmpty([]string{formatNode(n.Left), formatNode(n.Right)}, " && ")
	case KOr:
		return joinNonEmpty([]string{formatNode(n.Left), formatNode(n.Right)}, " || ")
	case KBg:
		return strings.TrimSpace(formatNode(n.Left)) + " &"
	case KBang:
		return "! " + formatNode(n.Left)
	case KSubshell:
		return "@ " + formatNode(n.Left)
	case KBrace:
		return "{ " + formatNode(n.Left) + " }"
	case KParen:
		return "(" + formatNode(n.Left) + ")"
	case KIf:
		return "if " + formatNode(n.Left) + " " + formatNode(n.Right)
	case KElse:
		return formatNode(n.Left) + " else " + formatNode(n.Right)
	case KWhile:
		return "while " + formatNode(n.Left) + " " + formatNode(n.Right)
	case KFor:
		name := formatNode(n.Left)
		if len(n.List) > 0 {
			list := formatWords(&Node{Kind: KWords, List: n.List})
			return fmt.Sprintf("for(%s in %s) %s", name, list, formatNode(n.Right))
		}
		return fmt.Sprintf("for(%s) %s", name, formatNode(n.Right))
	case KSwitch:
		return fmt.Sprintf("switch(%s){ %s }", formatNode(n.Left), formatNode(n.Right))
	case KCbody:
		return joinNonEmpty([]string{formatNode(n.Left), formatNode(n.Right)}, " ")
	case KCase:
		return "case " + formatWords(n.Left) + ";"
	case KMatch:
		return "~ " + formatNode(n.Left) + " " + formatNode(n.Right)
	case KCall:
		return formatCall(n)
	case KPre:
		return formatPre(n)
	case KEpilog:
		return formatWords(n)
	case KAssign:
		return formatNode(n.Left) + "=" + formatNode(n.Right)
	case KRedir:
		return formatRedir(n)
	case KDup:
		return formatDup(n)
	case KNmpipe:
		return formatNmpipe(n)
	case KArgs, KArgList, KWords:
		return formatWords(n)
	case KConcat, KWord, KVar, KFlat, KCount, KBackquote, KSub:
		return formatWord(n)
	default:
		return formatWord(n)
	}
}

func formatCall(n *Node) string {
	if n == nil {
		return ""
	}
	var parts []string
	if n.Left != nil {
		parts = append(parts, formatWords(n.Left))
	}
	if n.Right != nil {
		parts = append(parts, formatNode(n.Right))
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func formatWords(n *Node) string {
	if n == nil {
		return ""
	}
	if n.Kind == KArgList || n.Kind == KWords || n.Kind == KArgs {
		var parts []string
		for _, child := range n.List {
			if child == nil {
				continue
			}
			parts = append(parts, formatWord(child))
		}
		return strings.Join(parts, " ")
	}
	return formatWord(n)
}

func formatWord(n *Node) string {
	if n == nil {
		return ""
	}
	switch n.Kind {
	case KWord:
		return quoteIfNeeded(n.Tok)
	case KConcat:
		return formatWord(n.Left) + "^" + formatWord(n.Right)
	case KVar:
		if n.Left != nil {
			if n.Right != nil {
				return "$" + formatWord(n.Left) + "(" + formatWords(n.Right) + ")"
			}
			return "$" + formatWord(n.Left)
		}
		return "$"
	case KFlat:
		if n.Left != nil {
			return "$^" + formatWord(n.Left)
		}
		return "$^"
	case KCount:
		if n.Left != nil {
			return "$#" + formatWord(n.Left)
		}
		return "$#"
	case KSub:
		return formatWord(n.Left) + "(" + formatWords(n.Right) + ")"
	case KBackquote:
		if n.Left != nil {
			return "``" + formatWord(n.Left) + "{ " + formatNode(n.Right) + " }"
		}
		return "`{ " + formatNode(n.Right) + " }"
	default:
		return quoteIfNeeded(n.Tok)
	}
}

func formatRedir(n *Node) string {
	if n == nil {
		return ""
	}
	fd := ""
	if n.I1 >= 0 {
		fd = fmt.Sprintf("[%d]", n.I1)
	}
	return n.Tok + fd + " " + formatWord(n.Right)
}

func formatDup(n *Node) string {
	if n == nil {
		return ""
	}
	op := n.Tok
	if op == "" {
		op = ">"
	}
	if n.I2 < 0 {
		return fmt.Sprintf("%s[%d=]", op, n.I1)
	}
	return fmt.Sprintf("%s[%d=%d]", op, n.I1, n.I2)
}

func formatNmpipe(n *Node) string {
	if n == nil || n.Left == nil {
		return ""
	}
	fd := ""
	if n.Left.I1 >= 0 {
		fd = fmt.Sprintf("[%d]", n.Left.I1)
	}
	return n.Left.Tok + fd + "{ " + formatNode(n.Right) + " }"
}

func quoteIfNeeded(s string) string {
	if s == "" {
		return "''"
	}
	if !needsQuote(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func needsQuote(s string) bool {
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '#', ';', '&', '|', '^', '$', '=', '~', '`', '\'', '"', '{', '}', '(', ')', '<', '>', '[', ']':
			return true
		}
	}
	return false
}

func joinNonEmpty(parts []string, sep string) string {
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}

func formatPre(n *Node) string {
	var parts []string
	cur := n
	for cur != nil && cur.Kind == KPre {
		if cur.Left != nil {
			parts = append(parts, formatNode(cur.Left))
		}
		cur = cur.Right
	}
	if cur != nil {
		parts = append(parts, formatNode(cur))
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}
