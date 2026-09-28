# Instructions for AI agents working on this repository

This repo implements Veld, a programming language designed for AI agents.

- Read `CONTRIBUTING.md` (process and invariants) and `docs/DESIGN.md`
  (why things are the way they are) before changing the language.
- The language reference for *writing Veld code* is `docs/LANGUAGE.md`
  (also printed by `go run ./cmd/veld spec`).
- Build: `go build -o veld ./cmd/veld`. Go 1.22+, no third-party deps; keep it
  that way.
- Before finishing any change, run:
  ```sh
  go vet ./... && go test ./...
  go run ./cmd/veld fmt --check std examples evals
  ```
- Language changes need an RFC in `docs/rfcs/`. Bug fixes, better
  diagnostics, new fixes, examples and eval tasks do not.
- When you add a diagnostic code, document it in `internal/codes/codes.go`
  and add a case to `testdata/errors/` with a `# expect: CODE` header.
- Keep `docs/LANGUAGE.md` accurate: it is what other agents learn Veld from.
- Commit messages: imperative mood, one logical change per commit.
