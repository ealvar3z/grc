# grc beta status

`grc` is currently in beta.

The core language and execution semantics are stable, but some features are
intentionally incomplete.

---

## What works reliably

- Interactive REPL with history
- Ctrl-C handling for foreground jobs
- External programs, pipelines, redirections
- Background jobs and `jobs`
- rc-style expansion and functions
- `$status` and `$apid`

These features are considered stable.

---

## Known limitations

The following are not implemented:

- `if`, `for`, `switch`
- `fg` / `bg`
- job pruning
- `$apid` introspection helpers
- brace expansion
- `${}` parameter expansion
- here-documents

Pipelines involving builtins or functions may not participate fully in job
control (beta limitation).

---

## Safety notes

- Do not run `grc` as root.
- Signal handling and job control are implemented conservatively.
- Scripts written for `grc` should not assume POSIX shell behavior.

---

## Versioning

Breaking changes may occur until `v1.0.0`.
