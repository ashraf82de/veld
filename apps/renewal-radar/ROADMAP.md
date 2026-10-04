# Renewal Radar roadmap

## Current: usable local inventory

- [x] Add, list, filter, renew and remove tracked items.
- [x] Validate dates and persisted JSON.
- [x] Sort by expiry and calculate overdue/upcoming status.
- [x] Keep the previous data file as a backup.
- [x] Complete a recorded builder usability trial with realistic synthetic data.

## Next: safer data exchange

- CSV import with a dry-run validation report and duplicate detection.
- JSON export suitable for versioned backups.
- Category and owner filters.
- Tests covering interrupted-write recovery on Linux and Windows.

## Later: self-hosted web workflow

- Accessible server-rendered inventory and forms.
- A persistence design that prevents concurrent lost updates.
- Optional reminder adapters with explicit configuration and secret handling.
- Container and upgrade documentation after the server workflow is verified.

The next concrete action is a usability trial of the CLI followed by a focused
CSV import slice. Language gaps found during that work should be reproduced and
filed upstream with the exact Veld commit.
