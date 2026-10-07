# Changelog

All notable changes. Pre-1.0, minor versions may change syntax; every such
change lists how to migrate (`veld fix` carries the migration where possible).

## Unreleased (0.2.0)

### Maintenance
- Move repository maintenance to ChatGPT tasks using the connected GitHub app:
  daily development/triage and event-triggered PR review, with delegated
  merging after verification. Store portable task prompts in `docs/maintenance/`.
- Remove the three Claude Actions workflows and their model API key dependency.
  Keep nightly Linux/Windows CI, add manual dispatch, explicit read-only token
  permissions, bounded jobs and cancellation of superseded checks.
- Keep `.claude/` as an optional local adapter to the shared maintenance policy.

### Performance and memory
- New runtime core: functions are compiled to trees of Go closures. Variables
  live in frame slots, call targets and constructors are resolved once,
  `return`/`break`/`continue`/`?` are frame flags instead of panics, and
  Int/Float/Bool expressions run unboxed. Frames are pooled.
  `fib(32)`: 0.19s. A 20M-iteration loop: 0.67s.
- `List` and `Map` are persistent data structures (`internal/pds`): a 32-way
  vector trie and a hash-array-mapped trie with insertion order. Updates are
  O(log n) with structure sharing instead of O(n) copies (20k pushes + puts:
  10s to 0.08s).
- List slices (`list.take/drop/slice`, `[first, ..rest]` patterns) are O(1)
  views, with a copy fallback so a small slice never pins a large list.
  Recursion over a 50,000-element list with `[first, ..rest]`: 47s to 0.08s.
- `set xs = list.push(xs, x)` and `set xs = list.set_at(xs, ...)` on a local
  `var` update the list in place while that variable is provably its only
  holder, and freeze it on any read that could leak it. Values stay
  immutable to programs. A 2M-element sieve: 2.9s to 0.68s.
- The call depth limit is now 100,000 (was 10,000); runaway recursion is still a
  clean `R501` error.

### Added
- Record patterns (RFC 0002): `case User{name, age: 18, ..}`, nested in variants,
  with exhaustiveness. New diagnostics E510 (incomplete pattern, with a fix) and
  E511 (unknown record or field).
- In-place map updates: `set m = map.put(m, ...)`, `map.remove` and `map.update`
  on a local `var` mutate the map while it is provably unshared (same freeze
  rule as lists). 300k word-count updates: 0.33s to 0.19s.
- `std.cli` (argument parsing), `fs.read_lines`/`is_dir`/`rename`/`copy`,
  `io.read_lines`, `env.cwd`.
- Feedback and outreach: `veld report --feedback` / `--url` print a prefilled GitHub
  issue link; `veld eval export` writes the eval tasks as JSON lines;
  `llms.txt`, `docs/FOR_AGENTS.md`, a Hugging Face dataset card
  (`huggingface/`) and outreach drafts (`docs/outreach/`, for a human to post).
- Maintenance playbooks for improvements, dogfooding and outreach; scheduled
  execution now uses the ChatGPT tasks described above.
- `std.task.parallel_map`: run a function over a list on all cores, results in
  order; closures capturing a `var` are rejected at run time.
- `set s = s + a + b` on a local `var` appends into a buffer instead of copying,
  so building a string in a loop is linear (60,000 appends: 3.1s to 0.27s).
  Reads still see immutable strings.
- Tail calls: a call to a user function in tail position (last expression of a
  function, inside if/match branches, or `return f(x)`) reuses the caller's
  frame, so tail-recursive loops and mutual recursion run in constant stack.
  Frames of tail-called functions are omitted from stack traces; functions with
  `ensures` are excluded.
- `tests/fuzz_test.go`: fuzzers for the parser/formatter and the checker.
- Multi-value `match a, b` with `case p, q` (RFC 0001): state machines and
  routers without nested matches or throw-away records. New diagnostics
  E507 (wrong number of patterns, with a fix), E508 (more than 4 values), E509
  (commas in a single-value match).
- Effects `proc` and `state`; stdlib modules `regex`, `path`, `encoding`,
  `crypto`, `csv`, `datetime`, `process`, `state`; prelude `Pair[A, B]`;
  new functions in `str`, `list`, `map`, `math`, `json`, `http` (see
  docs/LANGUAGE.md). `examples/notes_api.veld` is a concurrent CRUD server.
- Eval tasks 009-025 (state machine, router, CSV report, regex, LRU cache,
  JSON summary, matrix, word wrap, shortest path, bank, brackets, intervals,
  run-length, binary tree, group report, dates, renewal queries).
- `veld report <files> -m "..."` prints a Markdown bug report (environment,
  source, diagnostics, test results) ready to paste into an issue.
- `case pattern =>` followed by a newline and an indented block is accepted
  (a common habit from other languages); `veld fmt` prints the canonical
  `case pattern` form.
- Maintainer instructions, optional local slash commands, issue forms, PR
  template, labels, cross-platform release workflow, and `docs/MAINTAINING.md`.
- `testdata/semantics/`: runtime behaviour tests for control flow, closures,
  patterns, recursion and value semantics; `bench/`: benchmark programs.

### Fixed
- A leading `and` or `or` on a new line now explains Veld's continuation rule
  and, when safe, offers a fix that moves the operator to the previous line.
- `veld fix --stdout` now checks intermediate repairs in memory and leaves
  source files untouched, including their modification times. In-place fixes
  report final write failures instead of silently succeeding.
- `veld fix` applied nothing when given an absolute Windows path.
- Applying several identical fixes (for example one `use std.option` per use of
  a missing module) duplicated the edit.
- Windows: CRLF sources no longer leak carriage returns into raw strings, and
  the repository forces LF so it checks out canonically formatted.

## 0.1.0

First release: parser with recovery and fixes, unification-based checker,
effects, exhaustiveness with witnesses, typed holes, contracts, interpreter,
standard library, formatter, test runner and agent tooling.
