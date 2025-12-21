Here is a **clean, senior-level `DESIGN.md` draft** you can drop directly into the repo. Itbs written to explain *why* `grc` looks the way it does, not just *what* it does, and it matches the architecture you actually built.

You can trim later, but I recommend starting with this full version.

---

# DESIGN.md

## grc b a Go implementation of the Plan 9 `rc` shell

`grc` is a Unix shell implemented in Go whose semantics are derived from the Plan 9 `rc` shell, as documented in `rc(1)`.
The goal of the project is **semantic fidelity**, not POSIX compatibility or feature parity with `bash`/`zsh`.

This document explains the architectural choices behind `grc`, the execution model, and the mapping from `rc` concepts to Go structures.

---

## Design goals

1. **Match `rc` semantics where they matter**

   * List-valued variables
   * Free carets
   * Cartesian concatenation
   * Dynamic scoping
   * Argument-level command substitution
   * `$status` propa.

2. **Keep parsing, expansion, and execution separate**

   * Avoid bstringly-typedb shell logic
   * Make each phase testable in isolation

3. **Prefer structural representations over ad-hoc execution**

   * Commands are not run directly from the AST
   * Everything passes through an explicit execution plan

4. **Remain debuggable**

   * Deterministic plan dumps
   * Execution tracing
   * No hidden implicit behavior

---

## High-level architecture

The shell is implemented as a **four-stage pipeline**:

```
Input
  b
Lexer / Parser
  b
AST
  b
Expansion
  b
Execution Plan
  b
Runner (execution)
```

Each stage has a clear responsibility and a stable interface.

---

## Parsing

### Grammar

* The grammar is derived from Plan 9 `rc` and implemented using `goyacc`.
* No execution or expansion happens during parsing.
* The parser builds a **pure structural AST**.

### AST

The AST is intentionally.

* Nodes represent *structure*, not behavior.
* Examples:

  * `KSeq` b sequencing (`;`)
  * `KPipe` b pipeline
  * `KConcat` b caret concatenation
  * `KDollar` b variable reference
  * `KBackquote` b command substitution
  * `KFnDef` b function definition
  * `KAssign` b assignment form

The AST is *not* executable.

---

## Expansion model

Expansion is the core semantic engine of `grc`.

### Expansion happens before execution

Expa.

Expansion is applied recursively and structurally:

* `$x` b list from environment
* `^` b list concatenation
* globbing b filesystem expansion
* backquotes b command substitution b list

### Expansion rules (rc-aligned)

* Variables are lists, not scalars.
* Concatenation rules:

  * Pairwise if lengths match
  * Distributive if one side has length 1
  * Error otherwise
* Globs that match nothing remain li.
* Quotes group words but do **not** suppress globbing.
* Backquotes produce lists split on whitespace.

Expansion is **pure** except for backquote execution, which is explicitly scoped.

---

## Environment model

The environment is a **dynamic chain of scopes**.

```go
type Env struct {
    parent *Env
    vars   map[string][]string
    funcs  map[string]FuncDef
}
```

### Key properties

* Variables are `[]string`
* Functions are stored alongside variables
* Lookup is dynamic (walks parent chain)
* Assignment prefixes create temporary child environments
* Functions execute in child environments

This mirrors `rc`bs dynamic scoping behavior.

---

## Execution plan

### Why a plan?

Directly executing the AST leads to:

* tangled logic
* difficult debugging
* implicit ordering

Instead, `grc` lowers the AST into an **explicit execution plan**.

### ExecPlan

An execution plan node represents *how* something will r.

```go
type ExecPlan struct {
    Kind        PlanKind
    Argv        []string
    Call        *parse.Node

    Prefix      []AssignPrefix
    Redirs      []RedirPlan

    PipeTo      *ExecPlan
    Next        *ExecPlan
    IfOK        *ExecPlan
    IfFail      *ExecPlan

    Background  bool
}
```

This allows:

* deterministic execution
* clean control-flow handling
* plan inspection and tracing

---

## Runner (execution)

The runner execu.

### Execution rules

* Expansion is performed **at runtime**, not at plan build time

  * Required for correct assignment behavior
* Prefix assignments apply only to the current command
* Builtins run in-process
* External commands use `os/exec`
* Pipelines use explicit pipes
* Redirections are applied structurally
* `$status` is updated after every command

### Builtins

Implemented builtins:

* `cd`
* `pwd`
* `exit`

Builtins:

* partic.
* respect redirections
* return status codes like external commands

---

## Debug and observability

`grc` includes first-class debugging support.

### Flags

* `-n` b parse and plan only, no execution
* `-p` b dump execution plan
* `-x` b trace executed commands

### Plan dump

Plan dumps are:

* deterministic
* stable
* human-readable
* suitable for tests

### Trace output

Trace shows **expanded argv**, not source text:

```
+ printf hi
```

This mirrors traditional shell tracing while remaining precise.

---

## Multi-line programs

The grammar parses a single command form.

Multi-line programs are handled by the **driver**, not the grammar:

* `ParseAll` repeatedly invokes the parser on the same lexer
* Forms are chained into a `KSeq` AST
* This avoids grammar complexity and keeps semantics clear

---

## Intentional omissions

The following are **out of scope** (for now):

* Job co.
* `$apid`
* Brace expansion
* `${}` parameter expansion
* POSIX compatibility
* Bash extensions

These can be added later, but are not required for rc correctness.

---

## Philosophy

`grc` is not a POSIX shell clone.

It is a **structural, list-oriented command language** that happens to execute Unix programs.

Design decisions prioritize:

* semantic clarity
* explicit structure
* faithful rc behavior
* debuggability

Over:

* convenience hacks
* compatibility tricks
* string-based execution

---

## Status

As of now, `grc` supports:

* rc argument semantics
* functions
* dynamic scoping
* assignments
* command substitution
* globbing
* full execution with pipes and redirections
* tracing and plan inspection

This is a stable foundation for further rc features or practical use.

