# Renewal Radar

Renewal Radar is a local, self-hostable tracker for contracts, licenses,
subscriptions, certificates and other items that must be renewed before an
expiry date. The application logic, persistence, validation and tests are
written in Veld.

This first usable slice is a command-line application with durable JSON
storage. It supports adding, listing, viewing, filtering, renewing and removing
items, plus validated CSV imports with duplicate detection.
Every write keeps a `.bak` copy of the previous data file. If a replacement is
interrupted after the primary is removed, the next read uses a complete,
validated `.tmp` file first and falls back to `.bak` when the temporary file is
partial or invalid.

## Run

From the Veld repository root:

```sh
go run ./cmd/veld run apps/renewal-radar/main.veld -- help
go run ./cmd/veld run apps/renewal-radar/main.veld -- add "Example domain" 2027-01-15 --owner=ops --category=domain
go run ./cmd/veld run apps/renewal-radar/main.veld -- list
go run ./cmd/veld run apps/renewal-radar/main.veld -- list --owner=ops --category=domain
go run ./cmd/veld run apps/renewal-radar/main.veld -- show 1
go run ./cmd/veld run apps/renewal-radar/main.veld -- due --days=60 --owner=ops
go run ./cmd/veld run apps/renewal-radar/main.veld -- health
go run ./cmd/veld run apps/renewal-radar/main.veld -- renew 1 2028-01-15
```

`list` and `due` accept optional `--owner` and `--category` filters. Values are
trimmed and compared without regard to letter case. Supplying both filters uses
AND semantics, so a record must match both.

`health` reads and validates the primary data file and its `.tmp` and `.bak`
companions, then reports which source normal commands would select. It never
creates, replaces or removes a file. An invalid selected primary or backup is
reported explicitly because normal commands will fail until it is repaired or
replaced.

Import an existing inventory with this exact header:

```csv
name,expires,owner,category,notes
Example domain,2027-01-15,ops,domain,primary domain
Support contract,2027-02-01,Ashraf,contract,"Annual, auto-renewing"
```

The same data is available in [`sample.csv`](sample.csv) for a quick trial.

Validate every record and check for duplicates without changing the data file,
then run the import:

```sh
go run ./cmd/veld run apps/renewal-radar/main.veld -- import renewals.csv --dry-run
go run ./cmd/veld run apps/renewal-radar/main.veld -- import renewals.csv
```

Records are duplicates when name, expiry, owner and category match after
trimming whitespace and ignoring case for text fields. Notes may differ without
making an otherwise identical renewal distinct. The import rejects the entire
file on the first error and writes nothing until every record passes.

Export a canonical, app-native JSON snapshot to a separate path:

```sh
go run ./cmd/veld run apps/renewal-radar/main.veld -- export backups/renewals.json
```

The destination uses the same validated format as the active data file, so it
can be restored by copying it over the active file while writers are stopped.
Repeated exports preserve the previous destination as `<json-path>.bak`. The
command refuses the active data file and its `.bak`/`.tmp` companions, and it
never changes the active inventory. Create the destination directory first.

The default data file is `renewal-radar.json` in the current directory. Use
`--file=PATH` on any command to choose another location. Dates accept
`YYYY-MM-DD` (treated as the end of that UTC day) or a full RFC 3339 timestamp.

## Verify

```sh
go run ./cmd/veld check apps/renewal-radar --json
go run ./cmd/veld test apps/renewal-radar --json
go run ./cmd/veld fmt --check apps/renewal-radar
```

The data file contains a JSON array and is intentionally portable. Copy the
primary file and its optional `.bak` companion for backups. A `.tmp` companion
is an interrupted replacement: when the primary is absent, a valid temporary
file takes precedence over the older backup; an invalid temporary file is
ignored and the backup is used. To restore manually, stop writers and copy the
chosen recovery file over the primary file. The CLI assumes one writer at a
time; shared multi-user access needs a storage design that prevents lost
updates.

See [PRODUCT.md](PRODUCT.md) for scope and acceptance criteria,
[ROADMAP.md](ROADMAP.md) for planned slices, and
[MAINTENANCE.md](MAINTENANCE.md) for the verified Veld version and handoff.
Use [INSTALL.md](INSTALL.md) for pinned source installation, backup, upgrade and
rollback instructions; no tagged Renewal Radar release has been published yet.
