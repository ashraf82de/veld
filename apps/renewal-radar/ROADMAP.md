# Renewal Radar roadmap

## Current: usable local inventory

- [x] Add, list, view, filter, renew and remove tracked items.
- [x] Validate dates and persisted JSON.
- [x] Sort by expiry and calculate overdue/upcoming status.
- [x] Keep the previous data file as a backup.
- [x] Complete a recorded builder usability trial with realistic synthetic data.
- [x] Import CSV with dry-run validation and duplicate detection.
- [x] Export canonical JSON snapshots suitable for versioned backups.
- [x] Filter list and due results by category and owner.
- [x] Recover a valid interrupted replacement, with backup fallback for a
  partial temporary file, on Linux and Windows.
- [x] Diagnose primary, temporary and backup validity and the selected recovery
  source without modifying files.

## Next: operational confidence

- [x] Prepare reproducible installation, backup, upgrade and rollback
  instructions for the first release candidate.
- [x] Complete a recorded source-install, side-by-side upgrade and code-only
  rollback trial against realistic synthetic data.
- [x] Prepare a repository-coordinated `0.1.0-rc.1` candidate with release
  notes, compatibility details and a source-integrity manifest.
- [x] Publish `renewal-radar-v0.1.0-rc.1` and verify the Linux AMD64 package,
  application checks, 18 tests, empty-inventory health behavior and archive
  checksum from the downloadable asset.
- [x] Publish corrected candidate `renewal-radar-v0.1.0-rc.2` with truthful
  in-package version text and GitHub prerelease metadata, then repeat the
  published-asset installation check.
- Collect a recorded usability trial from an independent operator and turn
  concrete findings into application fixes or upstream reports. Use the
  release-pinned, privacy-safe protocol in
  [`USABILITY_TRIAL.md`](USABILITY_TRIAL.md); preparing the protocol is not a
  completed trial.

## Later: self-hosted web workflow

- Accessible server-rendered inventory and forms.
- A persistence design that prevents concurrent lost updates.
- Optional reminder adapters with explicit configuration and secret handling.
- Container and upgrade documentation after the server workflow is verified.

The next concrete action is an independent recorded usability trial of the
published CLI. Language gaps found during that work should be reproduced and
filed upstream with the exact Veld commit; product findings belong in the
application backlog.
