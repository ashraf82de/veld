# Renewal Radar 0.1.0-rc.1

Status: **prepared, not published**. No tag, GitHub release or downloadable
artifact is claimed by this document. Repository tags publish Veld language
binaries as well as the application source, so publication must be coordinated
with the Veld maintainer.

## Candidate scope

This candidate is the first complete local CLI workflow. It can add, list,
view, filter, renew and remove renewals; validate and import CSV; export
canonical restorable JSON; diagnose data health; and recover a valid completed
temporary replacement with backup fallback.

Run `veld run apps/renewal-radar/main.veld -- version` to inspect the candidate
version and data-format compatibility without reading or writing the data file.

## Compatibility

- Data format `1` is the existing JSON array of renewal records. This candidate
  does not require a migration.
- The side-by-side trial recorded in [MAINTENANCE.md](MAINTENANCE.md) verified
  that the previous app-capable checkout could read a renewal written by the
  current checkout after a code-only rollback.
- Installation requires Git and Go 1.22 or newer to build the Veld runtime.
  Follow [INSTALL.md](INSTALL.md) with a reviewed full commit SHA.
- Stop all writers before backup, upgrade, rollback or manual restore. The
  application supports one writer and does not provide locking.

## Known limitations

- No browser interface, notifications, authentication or encryption.
- Data and notes are plaintext; do not store passwords, private keys or other
  secrets.
- The application has recorded synthetic usability, installation, upgrade and
  rollback trials, but no external-user adoption evidence yet.
- The application is distributed as source inside the Veld repository. No
  standalone Renewal Radar binary is currently produced.

## Verification required for publication

The exact candidate head must pass application check, all application tests,
repository formatting, race tests, Linux and Windows CI, all reference evals
and benchmark smoke tests. The publishing maintainer must record the full
40-character commit SHA, release URL and SHA-256 checksums for every published
artifact.

`SOURCE_SHA256SUMS` covers the first-party Veld source, project configuration
and sample import file. Verify it from the repository root with:

```sh
sha256sum -c apps/renewal-radar/SOURCE_SHA256SUMS
```

The manifest is source-integrity metadata, not a substitute for checksums of
future downloadable artifacts.
