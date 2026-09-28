---
description: Prepare a Veld release
argument-hint: "<version, e.g. 0.2.0>"
---

Use the `veld-maintainer` agent to prepare release $ARGUMENTS:

1. Confirm `main` is green (`gh run list --branch main --limit 3`).
2. Move the `Unreleased` section of `CHANGELOG.md` under `## $ARGUMENTS`, with
   today's date, and note migration steps for anything that changed syntax,
   diagnostic codes or stdlib names.
3. Update version mentions in `README.md`.
4. Run the full verification and the `bench/` programs; record the numbers in
   the changelog.
5. Open a PR titled `Release $ARGUMENTS`. Do NOT tag: a human tags `v$ARGUMENTS`
   after merging, which triggers the release workflow.
