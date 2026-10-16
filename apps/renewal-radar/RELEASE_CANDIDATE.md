# Renewal Radar 0.1.0-rc.2

Status: **prepared, not published**. Publication uses the application-specific
tag `renewal-radar-v0.1.0-rc.2`; plain `v*` tags are reserved for Veld language
releases. Tagging must be coordinated with the Veld maintainer.

The first published candidate, [0.1.0-rc.1](https://github.com/ashraf82de/veld/releases/tag/renewal-radar-v0.1.0-rc.1),
passed its release workflow and downloadable Linux installation trial, but its
version output incorrectly claimed that no tagged release existed and GitHub
did not mark it as a prerelease. Candidate rc.2 corrects both metadata defects;
it does not change data format or application behavior.

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
- Source installation requires Git and Go 1.22 or newer to build the Veld
  runtime. Published packages include a pinned runtime. Follow
  [INSTALL.md](INSTALL.md).
- Stop all writers before backup, upgrade, rollback or manual restore. The
  application supports one writer and does not provide locking.

## Known limitations

- No browser interface, notifications, authentication or encryption.
- Data and notes are plaintext; do not store passwords, private keys or other
  secrets.
- The application has recorded synthetic usability, installation, upgrade and
  rollback trials, but no external-user adoption evidence yet.
- Published packages contain the Veld source application and a pinned runtime;
  they are not standalone native rewrites of the application.

## Verification required for publication

The exact candidate head must pass application check, all application tests,
repository formatting, race tests, Linux and Windows CI, all reference evals
and benchmark smoke tests. The publishing maintainer must record the full
40-character commit SHA, release URL and SHA-256 checksums for every published
artifact. Versions containing a SemVer prerelease suffix such as `-rc.2` must
be marked as prereleases in GitHub.

The `release-renewal-radar` workflow verifies the source manifest and
application tests again, then packages the application source with a pinned
Veld runtime for Linux, macOS and Windows. Its release-level `SHA256SUMS` covers
the downloadable archives. The bundled runtime identifies both its Veld
development line and the Renewal Radar version; it is not a Veld language
release.

`SOURCE_SHA256SUMS` covers the first-party Veld source, project configuration
and sample import file. Verify it from the repository root with:

```sh
sha256sum -c apps/renewal-radar/SOURCE_SHA256SUMS
```

The manifest is source-integrity metadata, not a substitute for the published
archive checksums.
