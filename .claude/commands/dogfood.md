---
description: Write new programs in Veld like a first-time agent and turn every stumble into feedback
---

Use the `veld-maintainer` agent to dogfood Veld.

1. Build `veld`. Read ONLY `./veld spec` (the language guide), as a model that
   has never seen Veld would.
2. Pick two program ideas that differ from `examples/` and `evals/tasks/`: for
   example a CLI that reads stdin, a JSON or CSV transformer, a small HTTP
   service with `std.state`, a text-adventure state machine, a scheduler, a
   tokenizer, a matrix or graph algorithm. Prefer kinds of application the
   examples do not cover yet.
3. Write each program using only the guide and the toolchain
   (`veld check --json`, `veld fix`, `veld test`, `veld run`). Count the
   iterations, and note every diagnostic that was unclear, every standard
   library function you expected but did not find, every rule that surprised you.
4. For each stumble, open an issue labelled `agent-feedback` with the smallest
   reproducing program (use `veld report`), or, if the fix is small and safe,
   go straight to a PR (better diagnostic or fix, stdlib function with tests,
   guide clarification). Do not file duplicates: search issues first.
5. If a program came out well and is not a duplicate of an existing example, add
   it as an eval task (`evals/tasks/NNN-name/`, all four files, reference must
   pass) or an example, in a PR.
6. Finish with a table: program, iterations to green, stumbles, what you filed.
