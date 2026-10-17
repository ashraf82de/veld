# Renewal Radar maintenance

## Handoff

- Active product: Renewal Radar.
- Application location: `apps/renewal-radar/` in `ashraf82de/veld`.
- Initial Veld base: `0a20f1cf1581d0c43d7e6fe732ba37a28d1f3cd9`.
- CSV import Veld base: `70fd5aebfcbb7f7b753e44ee80ea66d57fb9569a`.
- JSON export Veld base: `530936af1a10428ac323eaff56ba0725b28ab8fe`.
- Owner/category filter Veld base: `51e0cf6dd5496b4163eb55abbb7e60c3f11cc3aa`.
- Interrupted-write recovery Veld base: `419447f10f64465f560f2f52d3cc04e6820c8c2d`.
- Data-health command Veld base: `fc14a56f6709e73c9067f6c623b0efbb4698364d`.
- Installation guidance Veld base: `bd5c48814f5a974104e626a095afef7656fe7689`.
- Upgrade/rollback trial Veld base: `7c18d999f079e4c94c50b6c1948b0ff5251123e5`.
- Release-candidate preparation Veld base: `c6a26b38f36ae1bd5e3e747394297848d62e941d`.
- Current state: the CLI supports its local inventory workflow and validated CSV
  import with duplicate detection and dry-run mode, canonical JSON export, and
  case-insensitive owner/category filters for list and due results, and
  validated temporary-file recovery with backup fallback. The read-only health
  command reports each companion's validity and the source normal reads select.
  Pinned source installation, backup, side-by-side upgrade and rollback steps
  are documented and verified. Corrected candidate `0.1.0-rc.2` is published
  as a GitHub prerelease, and its Linux AMD64 package passed checksum, source
  integrity, application-check, test, version and empty-health verification.
- Next action: collect a recorded usability trial from an independent operator
  and triage concrete product feedback before starting the later web workflow.
  Plain `v*` tags remain reserved for Veld language releases.
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
moves the temporary file into place. If the primary file is absent, reads use a
valid temporary file first; a partial or invalid temporary file is ignored and
the backup is used. Operators should still keep external versioned backups.

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

## Recorded interrupted-write trial: 2026-10-08

The builder simulated the two recoverable states left when replacement stops
after the primary file is removed. With a complete temporary JSON file and an
older backup, loading returned the complete temporary inventory. With a
truncated temporary file, loading rejected it and returned the valid backup.
Both tests removed their synthetic companion files afterward.

The valid-temporary test failed against the previous implementation because it
returned only the older backup, then passed after recovery began validating the
temporary file first. The application suite passed 12 tests; the full local
suite passed 64 Veld tests, all 25 reference evals and all nine benchmark smoke
programs. This is synthetic recovery evidence, not a claim about external user
adoption. No compiler or language defect was reproduced, so no upstream issue
was filed. Hosted Linux and Windows checks remain required before merging this
slice.

## Recorded data-health trial: 2026-10-09

The builder created a realistic synthetic domain renewal, retained a backup,
and ran the new `health` command. It reported a valid one-record primary, no
temporary file, a valid one-record backup and `primary` as the selected source.
SHA-256 checksums of both files were identical before and after the command,
and no extra file was created. Tests also cover valid temporary recovery,
invalid-temporary backup fallback and an empty inventory.

This is implementation and read-only-behavior evidence, not external adoption.
The application suite passed 17 tests; the full local suite passed 69 Veld
tests, all 25 reference evals and all nine benchmark smoke programs. The first
full-suite invocation omitted the freshly downloaded Go toolchain from `PATH`,
so the `std.process` test correctly reported that `go` was unavailable; the
documented verification passed after the toolchain directory was added. No
compiler or language defect was reproduced, so no upstream issue was filed.
Hosted Linux and Windows checks remain required before merging this slice.

## Recorded installation-guidance check: 2026-10-10

The builder used a fresh checkout pinned to
`bd5c48814f5a974104e626a095afef7656fe7689`, built a separate Veld binary from
source with Go 1.25.1, and ran the documented application check, tests and
initial health command against an empty external data directory. All 17
application tests passed, the health report selected an empty inventory and no
data file was created.

This verifies the source-build and initial-validation path, not a published
release or external adoption. A full side-by-side upgrade and rollback trial
remains the next product action. Linux and Windows CI are still required for
the documentation change itself.

## Recorded upgrade and rollback trial: 2026-10-11

The builder installed two fresh, detached source checkouts side by side: the
last application-changing main commit
`406c57324515f0d459732214552602986ccf8425` and current main
`7c18d999f079e4c94c50b6c1948b0ff5251123e5`. Both runtimes built from source
with Go 1.25.1, both application checks were clean and each checkout passed all
17 application tests.

Using the older checkout, the builder created realistic synthetic domain and
support-contract renewals in an external data directory. Health selected the
valid two-record primary, and the primary plus its one-record `.bak` companion
were copied to a separate backup directory. The current checkout reported the
same data health without changing the primary checksum, returned the expected
normalized owner/category query and renewed the support contract through
2028-03-01.

For the code-only rollback, the launch command was switched back to the older
checkout without restoring data. Its health command selected the current
two-record primary, and its list and filtered due query returned the renewal
written by the newer checkout. The external primary and backup checksums were
unchanged after the trial; the active `.tmp` companion was absent. This is
synthetic compatibility and recovery evidence, not a published release or
external user adoption. No compiler, language or application defect was
reproduced, so no upstream issue was filed. Hosted Linux and Windows checks
remain required before merging this record.

## Recorded release-candidate preparation: 2026-10-12

Candidate `0.1.0-rc.1` adds a read-only `version` command that reports the
application version, data format `1` and unpublished-candidate status. Running
it from an empty temporary directory produced the documented output without
creating a data file. Release notes now cover the supported workflow, data
compatibility, source installation, known limitations and the evidence still
required before publication.

`SOURCE_SHA256SUMS` verified every first-party Veld source file, the project
configuration and sample CSV. The application suite passed 18 tests; the full
local suite passed 70 Veld tests, all 25 reference evals and all nine benchmark
smoke programs with Go 1.25.1. This prepares the candidate but does not publish
it: no tag, GitHub release, downloadable artifact or external adoption is
claimed. Exact-head Linux and Windows CI remain required before merging, and
repository tagging must be coordinated with the Veld maintainer.

## Recorded published-package trial: 2026-10-15

The maintainer published
[`renewal-radar-v0.1.0-rc.1`](https://github.com/ashraf82de/veld/releases/tag/renewal-radar-v0.1.0-rc.1)
from exact main `6168b7a4d1a691073a7ed7162a80bc89da90d0ed`. Release workflow
[38693295420](https://github.com/ashraf82de/veld/actions/runs/38693295420)
passed source verification, application tests, cross-platform packaging and
publication.

The downloaded Linux AMD64 archive matched SHA-256
`fb6c437d2e1b83127facc5a5bc90b218fb3c4ccdfb40ea3ec440212c54ff62b6`.
Its source manifest, application check, all 18 tests and read-only
empty-inventory health trial passed. The trial then found that the packaged
`version` output still said no tag had been published and that GitHub recorded
the RC as a non-prerelease. Issue #28 tracks the correction. This is release
verification evidence, not external adoption.

## Recorded corrected-package trial: 2026-10-17

The builder published
[`renewal-radar-v0.1.0-rc.2`](https://github.com/ashraf82de/veld/releases/tag/renewal-radar-v0.1.0-rc.2)
from exact main `4ac8088235fa9b0c0d933091766b9e2e97fc741b`. Release workflow
[39006224073](https://github.com/ashraf82de/veld/actions/runs/39006224073)
passed the exact-main guard, source verification, application tests,
cross-platform packaging and publication. GitHub records the release with
`prerelease: true`.

Published artifact digests reported by GitHub are:

| Artifact | SHA-256 |
|----------|---------|
| `renewal-radar-0.1.0-rc.2-darwin-amd64.tar.gz` | `bc46f2e1ba23891ce0ff499d4a4ebace3c068dd545bbe0c2e19d2fa9a3b1a30e` |
| `renewal-radar-0.1.0-rc.2-darwin-arm64.tar.gz` | `ff039674d1e0805ae52730819caa64917fce1d697889b391bc57515c410583f5` |
| `renewal-radar-0.1.0-rc.2-linux-amd64.tar.gz` | `35762143c58c4796013bfd29a4dd1bd9a731d72ccc10a6d3b67a390d990efe50` |
| `renewal-radar-0.1.0-rc.2-linux-arm64.tar.gz` | `85bef18a7a77251e001633dfa4e57e42a4b6b88b0673b25a4bb4534d32fc4c22` |
| `renewal-radar-0.1.0-rc.2-windows-amd64.zip` | `c5bd156f20d2e7dc08ca7a58e73e9705287e1986eb09bbaf61e620729672f3b6` |
| `renewal-radar-0.1.0-rc.2-windows-arm64.zip` | `f37e330ba481e5a8534eaeeae34adb8d368efd7a0b689079bd7dac14674c6a54` |
| `SHA256SUMS` | `600077d2ea6aa0208c056c6de18b68e05abcf6d4e675fcf06d643f3d9255b205` |

The downloaded Linux AMD64 archive matched the published `SHA256SUMS`. Its
source manifest and application check passed, all 18 application tests passed,
and `version` reported `0.1.0-rc.2` with `Release status: release candidate`.
An empty-inventory `health` invocation selected an empty inventory and created
no data file. This closes the release-metadata reproduction in issue #28; it
is release verification evidence, not external-user adoption.
