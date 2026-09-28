## What and why

<!-- Lead with the result. Link the issue: "Fixes #123". -->

## How it was verified

- [ ] The failing case is now a test (golden diagnostic, `testdata/semantics`, `std` test or eval)
- [ ] `gofmt -l .`, `go vet ./...`, `go test ./...`
- [ ] `veld fmt --check std examples evals testdata/semantics bench` and `veld test std examples testdata/semantics`
- [ ] `docs/LANGUAGE.md`, `CHANGELOG.md`, `ROADMAP.md`, `internal/codes` updated if user-visible
- [ ] Runtime changes: `bench/` before/after numbers below

## Notes for the reviewer
