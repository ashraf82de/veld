---
description: Weekly maintenance pass: evals, benchmarks, backlog, digest issue
---

Use the `veld-maintainer` agent to run the weekly pass:

1. Build `veld`, run `./veld eval evals/tasks <refs> --json` against the
   reference solutions and every program in `bench/` (record wall time).
2. Read the three newest examples as a model seeing Veld for the first time and
   list every stumble.
3. Review open issues and PRs: close `needs-info` items quiet for 14+ days
   (explain why, invite reopening), ping stale PRs once, merge nothing that
   needs a human decision.
4. Pick at most one small, safe item from `ROADMAP.md` or the `agent-feedback`
   backlog and open a PR for it.
5. Open (or update) one issue titled `Weekly digest YYYY-MM-DD` with: bench
   numbers vs the previous digest, eval results, issues closed, PRs opened,
   and the three most valuable things a human could decide this week.
