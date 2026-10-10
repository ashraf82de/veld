# Install, upgrade and roll back Renewal Radar

Renewal Radar is currently distributed as Veld source in this repository. A
tagged Renewal Radar release has not been published yet. For a reproducible
installation, use a full Git commit SHA from a reviewed release candidate; do
not deploy a moving `main` checkout or `go install ...@latest`.

The application stores data separately from its source. Keep the checkout and
the data directory in different locations so code can be upgraded or rolled
back without moving the inventory.

## Requirements

- Git.
- Go 1.22 or newer to build the Veld runtime from source.
- One local writer at a time. Stop every writer before backup, upgrade,
  rollback or manual restore.
- A data directory that is backed up independently of the source checkout.

Linux and Windows run the full repository application tests in CI. The commands
below build from source so the runtime and application come from the same pinned
repository revision.

## Install from a pinned revision

Choose the full commit SHA recorded by the release candidate. Replace the
placeholder below; Git must resolve back to exactly the same SHA.

### Linux or macOS

```sh
export VELD_REF='<full-40-character-commit-sha>'
export RENEWAL_RADAR_HOME="$PWD/renewal-radar-install"
export RENEWAL_RADAR_DATA="$PWD/renewal-radar-data"

git clone https://github.com/ashraf82de/veld.git "$RENEWAL_RADAR_HOME"
git -C "$RENEWAL_RADAR_HOME" checkout --detach "$VELD_REF"
test "$(git -C "$RENEWAL_RADAR_HOME" rev-parse HEAD)" = "$VELD_REF"

mkdir -p "$RENEWAL_RADAR_HOME/bin" "$RENEWAL_RADAR_DATA"
(cd "$RENEWAL_RADAR_HOME" && go build -trimpath -o bin/veld ./cmd/veld)

"$RENEWAL_RADAR_HOME/bin/veld" check "$RENEWAL_RADAR_HOME/apps/renewal-radar" --json
"$RENEWAL_RADAR_HOME/bin/veld" test "$RENEWAL_RADAR_HOME/apps/renewal-radar" --json
"$RENEWAL_RADAR_HOME/bin/veld" run "$RENEWAL_RADAR_HOME/apps/renewal-radar/main.veld" -- health --file="$RENEWAL_RADAR_DATA/renewals.json"
```

The first `health` report should select `empty inventory` and must not create a
data file. Add a record only after the checkout SHA and verification results
are recorded.

### Windows PowerShell

```powershell
$env:VELD_REF = '<full-40-character-commit-sha>'
$env:RENEWAL_RADAR_HOME = Join-Path $PWD 'renewal-radar-install'
$env:RENEWAL_RADAR_DATA = Join-Path $PWD 'renewal-radar-data'

git clone https://github.com/ashraf82de/veld.git $env:RENEWAL_RADAR_HOME
git -C $env:RENEWAL_RADAR_HOME checkout --detach $env:VELD_REF
if ((git -C $env:RENEWAL_RADAR_HOME rev-parse HEAD) -ne $env:VELD_REF) { throw 'checkout SHA mismatch' }

New-Item -ItemType Directory -Force -Path (Join-Path $env:RENEWAL_RADAR_HOME 'bin'), $env:RENEWAL_RADAR_DATA | Out-Null
Push-Location $env:RENEWAL_RADAR_HOME
go build -trimpath -o bin/veld.exe ./cmd/veld
Pop-Location

& "$env:RENEWAL_RADAR_HOME/bin/veld.exe" check "$env:RENEWAL_RADAR_HOME/apps/renewal-radar" --json
& "$env:RENEWAL_RADAR_HOME/bin/veld.exe" test "$env:RENEWAL_RADAR_HOME/apps/renewal-radar" --json
& "$env:RENEWAL_RADAR_HOME/bin/veld.exe" run "$env:RENEWAL_RADAR_HOME/apps/renewal-radar/main.veld" -- health "--file=$env:RENEWAL_RADAR_DATA/renewals.json"
```

## Launch

Use the same explicit data path for every command. For example:

```sh
"$RENEWAL_RADAR_HOME/bin/veld" run "$RENEWAL_RADAR_HOME/apps/renewal-radar/main.veld" -- list --file="$RENEWAL_RADAR_DATA/renewals.json"
```

On Windows, use `veld.exe` and PowerShell's call operator as shown above. The
application is a CLI, not a daemon: this installation does not create a system
service, scheduled task, user account or network listener.

## Back up before a change

Stop writers, run `health`, then copy every companion that exists. On a POSIX
shell:

```sh
DATA_FILE="$RENEWAL_RADAR_DATA/renewals.json"
BACKUP_ROOT="$PWD/renewal-radar-backups"
BACKUP_DIR="$BACKUP_ROOT/$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$BACKUP_DIR"

"$RENEWAL_RADAR_HOME/bin/veld" run "$RENEWAL_RADAR_HOME/apps/renewal-radar/main.veld" -- health --file="$DATA_FILE"
for suffix in '' .bak .tmp; do
  if test -f "$DATA_FILE$suffix"; then
    cp -p "$DATA_FILE$suffix" "$BACKUP_DIR/renewals.json$suffix"
  fi
done
```

On Windows PowerShell:

```powershell
$dataFile = Join-Path $env:RENEWAL_RADAR_DATA 'renewals.json'
$backupRoot = Join-Path $PWD 'renewal-radar-backups'
$backupDir = Join-Path $backupRoot (Get-Date -AsUTC -Format 'yyyyMMddTHHmmssZ')
New-Item -ItemType Directory -Force -Path $backupDir | Out-Null

& "$env:RENEWAL_RADAR_HOME/bin/veld.exe" run "$env:RENEWAL_RADAR_HOME/apps/renewal-radar/main.veld" -- health "--file=$dataFile"
foreach ($suffix in @('', '.bak', '.tmp')) {
  $source = "$dataFile$suffix"
  if (Test-Path -LiteralPath $source -PathType Leaf) {
    Copy-Item -LiteralPath $source -Destination (Join-Path $backupDir "renewals.json$suffix")
  }
}
```

Keep external, versioned copies outside the data directory. The app-managed
`.bak` file is only the previous write, not a complete backup policy. Never put
passwords, private keys or other secrets in the plaintext notes field.

## Upgrade side by side

1. Stop every writer and create the backup above.
2. Clone the new reviewed commit into a new directory; do not overwrite the
   working installation.
3. Build the new runtime and run `check` and `test` using the install commands.
4. Run the new checkout's `health` command against the existing data path. It
   is read-only; review every primary, temporary and backup status and the
   selected source before continuing.
5. Read the candidate's release notes for data-format changes. The current
   candidate uses the existing JSON format and needs no migration.
6. Switch the launch command to the new checkout. Keep the previous checkout
   and the pre-upgrade backup until the new version has completed a real user
   workflow successfully.

Record the old and new full commit SHAs, verification output, backup location
and the time the launch command changed.

## Roll back

1. Stop every writer.
2. Run `health` with the current checkout and record its output.
3. If the release did not change the data format, point the launch command back
   to the previous pinned checkout and run its `health` command against the same
   data path.
4. Restore the pre-upgrade files only when data was damaged or the release
   notes require it. Preserve the current files separately first, then copy the
   backed-up primary and any `.bak`/`.tmp` companions back with writers stopped.
5. Run `health`, `list` and one read-only filtered or due query before allowing
   writes again.

Code rollback and data restore are separate decisions. Restoring old data
discards changes made since the backup, so switching code alone is preferred
when the format is compatible.

## First-release checklist

Before calling a revision a release candidate, record:

- the full Git SHA and intended version;
- clean application `check`, `test` and formatting results;
- complete Linux and Windows CI for that exact SHA;
- a source installation trial using this document;
- an upgrade and code-only rollback trial against realistic synthetic data;
- release notes covering data compatibility, known limitations and checksums
  for any published artifacts.

Publishing a Veld repository tag also publishes the language binaries, so the
application builder must not create a tag independently of the repository's
reviewed release process.
