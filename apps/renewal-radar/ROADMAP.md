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
- Prepare a repository-coordinated versioned release candidate with release
  notes, compatibility details and checksums for any published artifacts.

## Later: self-hosted web workflow

- Accessible server-rendered inventory and forms.
- A persistence design that prevents concurrent lost updates.
- Optional reminder adapters with explicit configuration and secret handling.
- Container and upgrade documentation after the server workflow is verified.

The next concrete action is release-candidate preparation coordinated with the
Veld repository's versioning and release process, unless product feedback
reveals a higher-priority defect. Language gaps found during that work should
be reproduced and filed upstream with the exact Veld commit.
