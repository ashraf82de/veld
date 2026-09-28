# Evals

These tasks measure how well AI agents write Veld. They drive the roadmap:
the language changes that matter are the ones that make these pass on the
first try.

Each task directory has:

- `prompt.md`: the task, including the exact public signature required
- `tests.veld`: hidden tests (`use solution`)
- `reference.veld`: a known-good solution (CI checks it passes)
- `veld.json`: marks the directory as the project root

## Running an agent against the suite

1. Give the agent the output of `veld spec` plus the task's `prompt.md`, and
   ask for the contents of `solution.veld`. Allow it to run `veld check --json`
   and `veld fix` on its draft if you are measuring iterations, or give it one
   shot if you are measuring first-try accuracy.
2. Save each answer as `<solutions-dir>/<task-name>.veld`
   (e.g. `sols/003-parse-pairs.veld`).
3. Grade:
   ```sh
   veld eval evals/tasks sols/          # table
   veld eval evals/tasks sols/ --json   # per-task results + error_code_counts
   ```

Track three numbers over time: first-try compile rate, first-try solve rate,
and mean check iterations to green. The `error_code_counts` histogram shows
which rules agents trip over; each frequent code is a candidate for a better
fix, a better guide section, or a language change (via RFC).

## Adding a task

Create `evals/tasks/NNN-short-name/` with the four files above. The task must
state the exact signature, and the reference must pass (`go test ./tests/`).
