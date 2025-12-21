grc conformance to rc(1)

This document records behavior relative to the Plan 9 rc(1) manual.
The goal is semantic conformance where practical; differences are noted.

Sources
- Plan 9 from User Space: rc(1) manual in ./docs

Status summary
- Parsing of core rc syntax: partial (documented below)
- Expansion rules ($, ^, free carets, glob): mostly aligned
- Control flow: if/for/while/switch implemented
- Job control: partial (jobs/fg/bg only; no full terminal control semantics)

Quoting and lexical rules
- Single quotes: implemented; '' inside single quotes becomes a single quote.
- Backslash-newline: treated as a blank (line continuation).
- Comments: # starts a comment to end-of-line, except inside single quotes.

Variables and lists
- Variables are list-valued.
- $name expands to a list; undefined expands to an empty list.
- $1..$n, $*, $0 implemented for function calls.
- $status is a list containing a numeric exit code.

Concatenation and free carets
- Free carets inserted between adjacent argument atoms without whitespace.
- ^ concatenation uses rc rules:
  - pairwise if lengths equal
  - distributive if one side length is 1
  - otherwise error

Globbing
- * ? [] patterns expanded after $ and ^.
- No-match patterns remain literal.

Backquote substitution
- `{...} command substitution supported (minimal).
- Output split on whitespace.
- $ifs not yet honored (default whitespace splitting only).

Control flow
- if (list) command implemented.
- if not command implemented, and now strict (requires preceding if).
- for(name in list) command implemented.
- for(name) command uses $* if list omitted.
- while(list) command implemented.
- switch(arg){case ...} implemented with rc-style fallthrough.
- ! operator implemented (status inversion).

Builtins
- cd, pwd, exit, jobs, fg, bg, apid implemented.
- exec, wait, shift, ., ~ not yet implemented.

Known gaps / mismatches
- Full quote/escape behavior is still partial (no double-quote semantics).
- $ifs not yet honored for backquote splitting.
- fd redirection syntax (>[2], dup) not implemented.
- Here documents and ${} expansion not implemented.

Conformance tests
Golden tests live under testdata/conformance and are executed by
internal/eval/conformance_test.go.
