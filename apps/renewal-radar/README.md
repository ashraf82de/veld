# Renewal Radar

Renewal Radar is a local, self-hostable tracker for contracts, licenses,
subscriptions, certificates and other items that must be renewed before an
expiry date. The application logic, persistence, validation and tests are
written in Veld.

This first usable slice is a command-line application with durable JSON
storage. It supports adding, listing, viewing, filtering, renewing and removing items.
Every write keeps a `.bak` copy of the previous data file, and a missing primary
file is recovered from that backup on the next read.

## Run

From the Veld repository root:

```sh
go run ./cmd/veld run apps/renewal-radar/main.veld -- help
go run ./cmd/veld run apps/renewal-radar/main.veld -- add "Example domain" 2027-01-15 --owner=ops --category=domain
go run ./cmd/veld run apps/renewal-radar/main.veld -- list
go run ./cmd/veld run apps/renewal-radar/main.veld -- show 1
go run ./cmd/veld run apps/renewal-radar/main.veld -- due --days=60
go run ./cmd/veld run apps/renewal-radar/main.veld -- renew 1 2028-01-15
```

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
primary file and its optional `.bak` companion for backups. To restore, stop
writers and copy the backup over the primary file. The CLI assumes one writer
at a time; shared multi-user access needs a storage design that prevents lost
updates.

See [PRODUCT.md](PRODUCT.md) for scope and acceptance criteria,
[ROADMAP.md](ROADMAP.md) for planned slices, and
[MAINTENANCE.md](MAINTENANCE.md) for the verified Veld version and handoff.
