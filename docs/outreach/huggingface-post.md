# Try Veld with your coding agent: help us find the rough edges

Can a language designed around compiler feedback make an agent's repair loop
easier? **Veld** is an open-source, pre-1.0 programming language exploring that
question. We are looking for agent developers and model evaluators willing to
try a small task and report both successes and failures.

Repository: https://github.com/ashraf82de/veld

## What your agent gets

- Named block endings such as `end fn` and `end match`, plus canonical formatting.
- Static types, immutable values, `Option` and `Result`, and exhaustive matches.
- Explicit effects and runtime effect denial.
- JSON diagnostics with stable codes, source spans and suggested edits.
- `veld spec` for the language guide, `veld describe` for library signatures,
  and `veld fix` for applying suggested repairs.

The goal is reliable generated code with fewer repair iterations. We do not
yet have evidence here that Veld outperforms established languages or that a
particular model performs better in Veld. It is an interpreter; syntax and APIs
may change before 1.0.

## A five-minute experiment

With Go 1.22 or later, install the current development version:

```sh
go install github.com/ashraf82de/veld/cmd/veld@main
veld version
veld spec
veld new app
veld check app --json
veld test app --json
```

Give your agent the guide from `veld spec` and one small task you care about.
Ask it to run the checker after each edit, add tests, and record its attempts.
For a suggested repair, `veld fix app/main.veld --stdout` previews the result;
`veld fix app/main.veld` applies it. Recent work
[fixed preview mode writing to source files](https://github.com/ashraf82de/veld/pull/7).

Start with the [agent guide](https://github.com/ashraf82de/veld/blob/main/docs/FOR_AGENTS.md)
or this [copyable agent invitation](https://github.com/ashraf82de/veld/blob/main/docs/outreach/agent-invitation.md).

## Help measure it

The repository has 25 [evaluation tasks](https://github.com/ashraf82de/veld/tree/main/evals/tasks)
covering tasks such as routing, text processing and data structures.
Give the model only the language guide and the selected task's `prompt.md`.
Keep `tests.veld` and `reference.veld` out of its context; they are public
repository files, so this is an evaluation protocol, not a secrecy guarantee.

Save generated answers as `sols/<task-name>.veld` and grade from a repository
checkout:

```sh
veld eval evals/tasks sols/ --json
```

Reference solutions passing establishes that the evaluator works. It does
not establish model performance. Please report first-attempt compile/solve
rates separately from results after repairs; record the repair loop's
diagnostics as well as the final evaluator output.

## Tell us what happened

[Open an agent-feedback issue](https://github.com/ashraf82de/veld/issues/new/choose),
or generate a prefilled report:

```sh
veld report your_file.veld -m "what worked or went wrong" --feedback
```

Review the generated report before submitting it; remove private code or data.
Useful reports include:

- Model name and version, agent framework, and available tools.
- Veld version and source commit, operating system, and the task prompt.
- Whether the first attempt compiled and passed tests.
- Number of check/edit cycles, the failing program, and relevant diagnostics.
- An unclear error, missing library function, or small reproducible bug.

A successful first attempt is useful feedback too. Contributions to examples,
diagnostics, regression tests and evaluation tasks are welcome. Language/API
proposals follow the repository's RFC process.

The Hub dataset export is prepared in
[huggingface/](https://github.com/ashraf82de/veld/tree/main/huggingface).
This invitation does not announce a published Hugging Face dataset.
