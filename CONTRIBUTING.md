# Contributing to Veld

Veld is developed largely by AI agents. This document is the contract every
change (human or agent) follows so the language keeps improving without
breaking.

## The loop

1. **Measure.** Run the eval suite against a model (see `evals/README.md`)
   and look at `error_code_counts` and failed tasks. Real failures from
   agents writing Veld are the best source of work.
2. **Propose.** Language changes (syntax, semantics, stdlib API, diagnostic
   codes) start as an RFC: copy `docs/rfcs/0000-template.md` to
   `docs/rfcs/NNNN-short-name.md` and open a PR. Bug fixes, better messages,
   new fixes and new evals do not need an RFC.
3. **Implement.** Keep changes small and focused.
4. **Verify.** All of these must pass:
   ```sh
   go vet ./...
   go test ./...
   go run ./cmd/veld fmt --check std examples evals
   go run ./cmd/veld test std examples
   ```
5. **Record.** Update `docs/LANGUAGE.md` (the agent guide), `ROADMAP.md`, and
   `internal/codes` for any new diagnostic.

## Invariants (never break these)

- `veld fmt` is idempotent and every file in `std/`, `examples/`, `evals/`
  is canonically formatted.
- Every diagnostic code emitted by the compiler is documented in
  `internal/codes/codes.go` (enforced by a test). Codes are never reused
  for a different meaning.
- A fix attached to a diagnostic must repair it when applied (tested for
  `testdata/errors`).
- The complete example in `docs/LANGUAGE.md` checks, passes its tests and is
  canonically formatted (enforced by a test).
- Every eval's `reference.veld` passes its `tests.veld`.
- There is still exactly one way to write each construct. A proposal that
  adds a second way must remove the first.

## Where things live

| Change                         | Files                                                         |
|--------------------------------|---------------------------------------------------------------|
| New syntax                     | `internal/syntax/{lexer,parser,ast}.go`, `internal/format`, interpreter, checker |
| Type or effect rule            | `internal/types/*.go`, a case in `testdata/errors/`           |
| New stdlib function            | `std/<module>.veld` (prefer Veld); `extern` + `internal/interp/natives.go` if native |
| New diagnostic                 | emit it, document it in `internal/codes`, add a golden case   |
| New eval task                  | `evals/tasks/NNN-name/{prompt.md,tests.veld,reference.veld,veld.json}` |

## Writing good diagnostics

A diagnostic should answer: what is wrong, where exactly, and what to write
instead. Prefer a `Fix` (text edits) over a note; prefer a note over nothing.
Mention the Veld equivalent when the mistake is a habit from another language.
