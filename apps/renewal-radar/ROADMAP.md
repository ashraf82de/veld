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

## Next: operational confidence

- Add a read-only data health command that reports the primary, temporary and
  backup state and the recovery source without modifying files.
- Prepare installation and first-release instructions after the health command
  is verified.

## Later: self-hosted web workflow

- Accessible server-rendered inventory and forms.
- A persistence design that prevents concurrent lost updates.
- Optional reminder adapters with explicit configuration and secret handling.
- Container and upgrade documentation after the server workflow is verified.

The next concrete action is a read-only data health command unless product
feedback reveals a higher-priority defect. Language gaps found during that
work should be reproduced and filed upstream with the exact Veld commit.
