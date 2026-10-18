# Independent usability trial

This protocol lets an operator who did not build Renewal Radar evaluate the
published CLI consistently. It is a research aid, not a script for producing a
positive result. Record failures, abandoned tasks and confusing output exactly
as they occur. Do not get implementation help until the unaided tasks finish.

Use synthetic renewal metadata only. Renewal Radar stores plaintext; never put
passwords, private keys, customer names or other sensitive data in the trial.

## Trial identity

Record these before starting:

| Field | Value |
|---|---|
| Date (UTC) | |
| Operator role (no name required) | |
| Operating system and architecture | |
| Renewal Radar release | `0.1.0-rc.2` |
| Archive filename | |
| Archive SHA-256 | |
| Prior use of Renewal Radar | none / some |
| Prior use of command-line tools | low / medium / high |

Use the archive and `SHA256SUMS` from the
[`renewal-radar-v0.1.0-rc.2` release](https://github.com/ashraf82de/veld/releases/tag/renewal-radar-v0.1.0-rc.2).
Verify the archive as described in [INSTALL.md](INSTALL.md), extract it into a
new directory and do not use an existing inventory.

## Rules for the facilitator

- Give the operator the extracted package and this document only.
- Let the operator consult `README.md`, `INSTALL.md` and command help.
- Do not explain command syntax or recover from errors during the unaided run.
- Ask the operator to think aloud, but do not turn observations into leading
  questions.
- Record exact commands and error text. Replace local directory names if they
  are sensitive; do not paraphrase diagnostics.
- Stop a task after ten minutes or when the operator chooses to abandon it.

## Tasks

Use one new data path for every task, such as
`$PWD/trial-data/renewals.json`. The expected outcome describes the product
result, not the command the operator must discover.

1. Confirm which Renewal Radar version is installed.
2. Check that the new inventory is healthy without creating the data file.
3. Add these two synthetic records:
   - Example domain, expires `2027-01-15`, owner `ops`, category `domain`.
   - Support contract, expires `2027-03-01`, owner `alex`, category
     `contract`, notes `Annual renewal`.
4. Find all records owned by `ops` and confirm only the example domain appears.
5. Find records due within 120 days. State which records appear and why; the
   answer depends on the trial date.
6. Change the support contract expiry to `2028-03-01`.
7. Export a restorable JSON snapshot to a different directory.
8. Check data health again and identify the selected source and backup state.
9. Explain, using the supplied documentation, how to restore the exported
   snapshot without performing the restore.

Do not correct a failed task and then mark it successful. A task can be
`completed`, `completed with help`, `failed` or `abandoned`.

## Observation record

| Task | Result | Time | Attempts | Exact friction or surprising output |
|---|---|---:|---:|---|
| 1 | | | | |
| 2 | | | | |
| 3 | | | | |
| 4 | | | | |
| 5 | | | | |
| 6 | | | | |
| 7 | | | | |
| 8 | | | | |
| 9 | | | | |

After the unaided tasks, ask:

1. What did you expect to be easier?
2. Which output or term was least clear?
3. What would stop you using this for a real, non-sensitive inventory?
4. What is the smallest change that would make you try it again?

Record answers as the operator's feedback, not as confirmed defects.

## Triage and evidence

Before filing anything, reproduce the observation with the same release and a
fresh synthetic data file. Search existing issues first.

- Product behavior, workflow and documentation findings belong in a Veld
  repository issue that names `apps/renewal-radar` and the release.
- A compiler, runtime or diagnostic problem needs a minimal standalone Veld
  reproducer, exact Veld commit, command, expected result and actual result;
  apply the `agent-feedback` label.
- A preference or untested hypothesis belongs in the application backlog, not
  a bug report.

Attach the completed observation table, sanitized commands and output, OS and
architecture, release name and verified archive digest. State whether each
finding was reproduced. Do not attach the inventory or exported snapshot.

A completed trial is evidence about one operator and one environment. It is
not evidence of adoption, model performance or general usability.
