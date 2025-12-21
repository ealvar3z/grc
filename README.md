# grc — a Go implementation of the Plan 9 rc shell

`grc` is an interactive and scripting shell for Unix systems, implemented in Go,
with semantics derived from the Plan 9 `rc` shell.

Unlike POSIX shells, `rc` is a list-oriented command language. Variables hold
lists, expansion is structural, and control flow is explicit. `grc` aims to
preserve these semantics while running on a modern Unix system.

This project is in beta, but is usable as a daily interactive shell.

---

## Features

- List-valued variables
- `$` expansion with rc semantics
- Free carets and `^` concatenation
- Globbing with literal fallback
- Backquote command substitution
- Functions with dynamic scoping
- Assignments (standalone and prefix)
- Pipes, redirections, background jobs
- `$status` and `$apid`
- Interactive REPL with history and prompt
- Execution tracing and plan inspection

---

## Non-goals

`grc` is not a POSIX shell and does not aim to be compatible with:
- bash / zsh extensions
- POSIX parameter expansion
- brace expansion
- job control beyond minimal `jobs`

---

## Installation

```sh
go build ./cmd/grc
```

Run interactively:

```sh
./grc
```

Run a script:

```sh
./grc < script.rc
```

Interactive flags

- -n parse and plan only (no execution)
- -p print execution plan
- -x trace executed commands

These flags work in both script and interactive modes.

Documentation

- DESIGN.md — architecture and execution model
- CONFORMANCE.md — mapping to Plan 9 rc(1)
- USAGE.md — practical examples
- BETA.md — beta status and limitations

Status

grc is stable enough for daily interactive use, but some rc features are still
in progress.

Do not run as root.

License

MIT
