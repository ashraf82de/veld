---
license: mit
language:
  - en
pretty_name: Veld evals
task_categories:
  - text-generation
tags:
  - code
  - code-generation
  - programming-language
  - agents
  - benchmark
size_categories:
  - n<1K
configs:
  - config_name: default
    data_files:
      - split: test
        path: veld-evals.jsonl
---

# Veld evals

Programming tasks for **Veld**, a statically typed language designed to be
written by AI agents ([github.com/ashraf82de/veld](https://github.com/ashraf82de/veld)).
Each task has a prompt with the exact public signature, hidden tests written in
Veld, and a reference solution. The tasks measure how easily an agent writes
correct Veld from the language guide and the compiler's feedback: first-try
compile rate, first-try solve rate, and iterations to green.

## Fields

| field | meaning |
|-------|---------|
| `task_id` | e.g. `010-router` |
| `prompt` | the task, including the required signature |
| `tests` | hidden tests (`use solution`, then `test` blocks) |
| `reference` | a known-good `solution.veld` |
| `language` | always `veld` |
| `version` | the Veld version the file was exported with |

## Use

1. Install Veld: `go install github.com/ashraf82de/veld/cmd/veld@latest`.
2. Give a model `veld spec` (the language guide) and a task's `prompt`; ask for
   the contents of `solution.veld`. To measure iterations, let it call
   `veld check --json` and `veld fix` on its draft.
3. For each task, create a directory with `veld.json` (`{"name": "task"}`),
   `tests.veld` and the model's `solution.veld`, or use the repository's own
   runner: save answers as `sols/<task_id>.veld` and run
   `veld eval evals/tasks sols/ --json`. The JSON report includes a histogram of
   the diagnostic codes the model triggered.

## Feedback

If a model struggles with Veld, we want to know: run
`veld report your_file.veld -m "what went wrong" --feedback` and open the link it
prints, or open an issue at
[github.com/ashraf82de/veld/issues](https://github.com/ashraf82de/veld/issues).
The dataset grows with that feedback; the repository is at 25 tasks today.

## License

MIT. The reference solutions and tests are part of the Veld repository.
