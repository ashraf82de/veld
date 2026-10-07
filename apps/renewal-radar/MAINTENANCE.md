# Renewal Radar maintenance

## Handoff

- Active product: Renewal Radar.
- Application location: `apps/renewal-radar/` in `ashraf82de/veld`.
- Initial Veld base: `0a20f1cf1581d0c43d7e6fe732ba37a28d1f3cd9`.
- CSV import Veld base: `70fd5aebfcbb7f7b753e44ee80ea66d57fb9569a`.
- JSON export Veld base: `530936af1a10428ac323eaff56ba0725b28ab8fe`.
- Owner/category filter Veld base: `51e0cf6dd5496b4163eb55abbb7e60c3f11cc3aa`.
- Current state: the CLI supports its local inventory workflow and validated CSV
  import with duplicate detection and dry-run mode, canonical JSON export, and
  case-insensitive owner/category filters for list and due results.
- Next action: add interrupted-write recovery coverage on Linux and Windows
  unless user feedback exposes a higher-priority defect.
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

## Recorded import trial: 2026-10-05

The builder imported realistic synthetic domain and support-contract records
from CSV after first running `--dry-run`. The dry run left the data file absent;
the real import wrote both records, preserved an existing record when tested,
and the list command showed the combined inventory in expiry order. Invalid
headers, column counts, names, dates and duplicates were rejected by tests.

This is an implementation trial rather than external adoption. The first check
reported three `E104` errors because a boolean expression continued on new
lines beginning with `and`; naming the four boolean parts cleared the errors in
one edit. The checker, eight application tests and the complete local repository
verification then passed. No compiler or language defect was reproduced, so no
upstream issue was filed. Hosted Linux and Windows checks remain required before
merging this slice.

## Recorded export trial: 2026-10-06

The builder added two realistic synthetic renewals, exported the active data to
a separate JSON file, and verified that the export was byte-for-byte identical
while the source checksum remained unchanged. After renewing one record, a
second export matched the updated source and preserved the first export as
`<destination>.bak`. Copying the new export to a fresh data path restored both
records. A normalized path alias of the active data file was rejected without
changing the source.

This is recovery-workflow evidence, not external adoption. Importing `std.path`
initially produced `E204` errors because existing bindings were also named
`path`; renaming those bindings cleared every diagnostic in one edit. Nine
application tests then passed. No compiler or language defect was reproduced,
so no upstream issue was filed. Hosted Linux and Windows checks remain required
before merging this slice.

## Recorded filtering trial: 2026-10-07

The builder used a realistic synthetic inventory with domain and contract
renewals assigned to operations and sales owners. Owner-only and category-only
queries returned the expected subsets; combining both filters returned their
intersection. Mixed-case input and surrounding whitespace matched normalized
stored values, and the same filters constrained a due-date query. An unmatched
owner returned an empty inventory rather than unrelated records.

This is an implementation trial, not external user feedback or adoption. The
application tests cover independent and combined filters, normalization, empty
filter rejection, unmatched values and composition with the due window. No
compiler or language defect was reproduced, so no upstream issue was filed.
Hosted Linux and Windows checks remain required before merging this slice.
