# Changelog

## Unreleased
- Initialized Go module `grc` with entrypoint `cmd/grc/main.go` and module file `go.mod`.
- Added parser scaffolding under `internal/parse/` including AST kinds/nodes, list helpers, parser grammar, lexer, parse entrypoint, and tests.
- Added lexer position tracking, single-quoted word handling, and direct lexer tests.
- Copied upstream reference artifacts into `upstream/` and added `upstream/go.mod` to keep them out of `go test ./...`.
- Renamed top-level `syn.y` and `y.go` to `syn.y.txt` and `y.go.txt` to avoid Go toolchain parsing errors.
- Added `.gitignore` to ignore `.gocache/`.
- Updated pipe parsing to use literal `|` and regenerated `internal/parse/parser.go`.
