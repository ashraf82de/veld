# Veld for AI agents

Everything an agent (or the person running it) needs to try Veld in five
minutes and to tell us how it went.

## 1. Get the tool

```sh
go install github.com/ashraf82de/veld/cmd/veld@latest
veld version
```

No Go? Download a binary for Linux, macOS or Windows from the
[releases page](https://github.com/ashraf82de/veld/releases).

## 2. Learn the language

Put the output of `veld spec` (about 400 lines, roughly 4,000 tokens) in your
context. It is the complete reference: syntax, types, effects, the standard
library, and a translation table from habits in other languages.

## 3. Work in a loop

```sh
veld new app                      # starter project
veld check app --json             # diagnostics: stable codes, exact spans, fixes
veld fix app/main.veld            # apply the suggested fix of every error
veld test app --json              # run `test` blocks written next to the code
veld run app/main.veld            # run main()
veld fmt app                      # the one canonical layout
veld describe std.list            # signatures and docs of a module
veld explain E303                 # what a diagnostic means
```

Write `???` for any expression you have not decided yet: the checker reports
the type it needs and the variables in scope. Untrusted programs can be run
with `veld run --deny net,fs,env,proc,state,rand,time file.veld`; the runtime
enforces the denial.

## 4. A prompt you can copy

> You will write a program in Veld, a statically typed language for AI agents.
> Read this guide first: <paste `veld spec`>. Work only with the `veld` CLI:
> run `veld check --json` after every edit and `veld fix` for mechanical
> errors, put tests in `test` blocks, and finish with `veld test`. Task: ...

## 5. Try the evals

`evals/tasks/` has 24 programming tasks with hidden tests (also published as a
dataset, see `huggingface/README.md`). Give an agent the guide plus a task's
`prompt.md`, save its `solution.veld` as `sols/<task>.veld`, then:

```sh
veld eval evals/tasks sols/ --json
```

You get per-task results, and a histogram of the error codes the agent hit,
which is the most useful signal we have for what to improve.

## 6. Tell us how it went

Feedback from agents and the people who run them decides what is built next.
Any of these is welcome, including "it worked first try":

- **One command:** `veld report your_file.veld -m "what went wrong" --feedback`
  prints a link that opens the *Agent feedback* issue form with your program,
  diagnostics and environment already filled in. Open it and submit.
- **Bug in the tool:** same command with `--url` instead of `--feedback`.
- **Just tell us:** [open an issue](https://github.com/ashraf82de/veld/issues/new/choose).

Useful details: the model and setup, the task, how many iterations it took,
which diagnostics were confusing, which standard-library function you expected
to exist. Every new issue is triaged by the maintainer agent, and every
frequent friction becomes an eval task or a better diagnostic.
