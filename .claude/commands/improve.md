---
description: Pick the most valuable next improvement to Veld and ship it as a PR
---

Use the `veld-maintainer` agent to make Veld measurably better today.

1. Sync (`git pull --ff-only`) and look at what is already in flight:
   `gh pr list --state open --search "head:agent/"`. If three or more agent PRs
   are open, do not open another: review them, rebase or fix failing ones,
   and stop.
2. Choose ONE item, in this order of preference:
   1. an open `agent-feedback` issue (three reports of the same friction
      outrank everything else);
   2. an open P0/P1 bug;
   3. the first unchecked item under "Next" in `ROADMAP.md` that is small enough
      to finish, test and document in one PR;
   4. a gap you found yourself by reading the newest example as if you were a
      model seeing Veld for the first time.
3. Write the failing test first (golden diagnostic, semantics test, std test or
   eval task), then the smallest change that fixes it. Language changes need an
   RFC in `docs/rfcs/` instead of code: write it and stop.
4. Run the full verification from `docs/MAINTAINING.md` and, for runtime work,
   the `bench/` programs before and after.
5. Update `docs/LANGUAGE.md`, `CHANGELOG.md`, `ROADMAP.md` and
   `internal/codes/codes.go` as needed; refresh the numbers in
   `docs/outreach/` and `README.md` if benchmarks changed.
6. Open a PR on `agent/improve-<slug>` and describe: what improved, how it was
   measured, what a reviewer should look at first. Comment on the issue it
   addresses.

If nothing is worth doing today, say so in one line; that is a fine outcome.
