package eval

import (
	"fmt"
	"strings"
)

// DumpPlan returns a readable representation of an execution plan.
func DumpPlan(p *ExecPlan) string {
	var b strings.Builder
	dumpPlan(&b, p, 0)
	return b.String()
}

func dumpPlan(b *strings.Builder, p *ExecPlan, indent int) {
	if p == nil {
		return
	}
	pad := strings.Repeat(" ", indent)
	fmt.Fprintf(b, "%s- %s\n", pad, planLine(p))
	if p.PipeTo != nil {
		fmt.Fprintf(b, "%s  PIPE->\n", pad)
		dumpPlan(b, p.PipeTo, indent+4)
	}
	if p.IfOK != nil {
		fmt.Fprintf(b, "%s  IFOK->\n", pad)
		dumpPlan(b, p.IfOK, indent+4)
	}
	if p.IfFail != nil {
		fmt.Fprintf(b, "%s  IFFAIL->\n", pad)
		dumpPlan(b, p.IfFail, indent+4)
	}
	if p.Next != nil {
		fmt.Fprintf(b, "%s  NEXT->\n", pad)
		dumpPlan(b, p.Next, indent+4)
	}
}

func planLine(p *ExecPlan) string {
	parts := []string{planKindName(p.Kind)}
	if len(p.Argv) > 0 {
		parts = append(parts, "argv="+strings.Join(p.Argv, " "))
	}
	if len(p.Prefix) > 0 {
		var pref []string
		for _, pr := range p.Prefix {
			pref = append(pref, pr.Name)
		}
		parts = append(parts, "prefix="+strings.Join(pref, ","))
	}
	if p.AssignName != "" {
		parts = append(parts, "assign="+p.AssignName)
	}
	if p.Func != nil {
		parts = append(parts, "func="+p.Func.Name)
	}
	if p.ForName != "" {
		parts = append(parts, "var="+p.ForName)
	}
	if p.Background {
		parts = append(parts, "bg")
	}
	if len(p.Redirs) > 0 {
		parts = append(parts, "redirs="+formatRedirs(p.Redirs))
	}
	return strings.Join(parts, " ")
}

func planKindName(k PlanKind) string {
	switch k {
	case PlanCmd:
		return "CMD"
	case PlanFnDef:
		return "FNDEF"
	case PlanFnRm:
		return "FNRM"
	case PlanNoop:
		return "NOOP"
	case PlanAssign:
		return "ASSIGN"
	case PlanIf:
		return "IF"
	case PlanFor:
		return "FOR"
	case PlanWhile:
		return "WHILE"
	case PlanSwitch:
		return "SWITCH"
	case PlanNot:
		return "NOT"
	case PlanSubshell:
		return "SUBSHELL"
	case PlanTwiddle:
		return "MATCH"
	default:
		return "UNKNOWN"
	}
}

func formatRedirs(rs []RedirPlan) string {
	var parts []string
	for _, r := range rs {
		if r.Op == "dup" {
			if r.Close {
				parts = append(parts, fmt.Sprintf("dup:%d=", r.Fd))
			} else {
				parts = append(parts, fmt.Sprintf("dup:%d=%d", r.Fd, r.DupTo))
			}
			continue
		}
		fd := ""
		if r.Fd >= 0 {
			fd = fmt.Sprintf("%d", r.Fd)
		}
		if len(r.Target) == 0 {
			parts = append(parts, fd+r.Op+":")
			continue
		}
		parts = append(parts, fd+r.Op+":"+strings.Join(r.Target, ","))
	}
	return strings.Join(parts, ",")
}
