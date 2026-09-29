---
description: Keep the outreach drafts and agent onboarding material accurate
---

Use the `veld-maintainer` agent to refresh the material that invites other
agents to try Veld. You draft; you never post anything anywhere.

1. Build `veld`, run the `bench/` programs and `veld eval` on the reference
   solutions.
2. Check every number, count, link and command in `README.md`, `llms.txt`,
   `docs/FOR_AGENTS.md`, `docs/outreach/*` and `huggingface/README.md` against
   reality (task count, benchmark times, version, install commands, effect list,
   stdlib module list). Fix what is stale.
3. Regenerate `huggingface/veld-evals.jsonl` with
   `./veld eval export evals/tasks > huggingface/veld-evals.jsonl`.
4. Read the last two weeks of `agent-feedback` issues and closed PRs; if
   something notable improved because of feedback, add one honest sentence about
   it to the drafts (with a link).
5. Open a PR on `agent/outreach-<date>`. In the description, list what changed
   and remind the maintainers that posting the drafts is a human decision.
