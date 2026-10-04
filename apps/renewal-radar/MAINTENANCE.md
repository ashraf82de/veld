# Renewal Radar maintenance

## Handoff

- Active product: Renewal Radar.
- Application location: `apps/renewal-radar/` in `ashraf82de/veld`.
- Initial Veld base: `0a20f1cf1581d0c43d7e6fe732ba37a28d1f3cd9`.
- Current state: first CLI vertical slice passes local verification and awaits
  hosted Linux and Windows CI.
- Next action: implement CSV import as one focused change unless hosted CI or
  user feedback exposes a higher-priority defect.
- Upstream issues: none filed. No compiler or language defect has yet been
  reproduced by this application.

## Verification

Run from the Veld repository root:

```sh
go vet ./...
go test -race ./...
go build -o veld ./cmd/veld
./veld fmt --check std examples evals testdata/semantics bench apps/renewal-radar
./veld test std examples testdata/semantics apps/renewal-radar
./veld check apps/renewal-radar --json
```

Record the exact application commit and Veld commit for every trial or bug
report. The application tests use synthetic records to verify behavior; they
are not evidence of user adoption.

## Data and recovery

The CLI writes one JSON array. Before replacing an existing file, it copies the
previous contents to `<file>.bak`, writes the new contents to `<file>.tmp`, and
moves the temporary file into place. If the primary file is absent, reads fall
back to the backup. Operators should still keep external versioned backups.

The application does not provide locking. Do not run concurrent writers against
the same file. Do not store passwords, private keys or other secrets in notes.

## Recorded trial: 2026-10-04

The builder ran the complete CLI journey against a temporary file using two
realistic synthetic records: a domain and a support contract. Adding both
records, ordered listing, a 90-day due filter, renewing the contract, removing
the domain and listing the remaining record all produced the expected output.
The primary JSON file and previous-file backup both existed after the writes.

This was an implementation trial, not external user feedback or adoption. On
the first generated code pass, `veld check --json` reported one shadowing error,
ten unnamed-argument errors and one unused-binding warning. `veld fix` repaired
all unnamed arguments; two small name edits cleared the rest. The second check
passed, and five application tests passed. No compiler or language defect was
reproduced, so no upstream issue was filed.
