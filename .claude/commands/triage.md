---
description: Triage open GitHub issues (or one issue) for the Veld repo
argument-hint: "[issue number]"
---

Use the `veld-maintainer` agent to triage $ARGUMENTS (all open issues that have
no `type:*` label if no number is given).

For each issue: classify it, add `type:*`, `area:*` and `P0`-`P3` labels,
reproduce it (`veld check`/`veld test`, or `veld run --deny net,fs,env,time,rand`
for programs from reporters), and reply once with either a clear next step or
one concrete question that points at `veld report`. Do not change code in this
command. Finish with a table: issue, classification, priority, next step.
