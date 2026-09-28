# Roadmap

Priorities are set by the eval suite: the most frequent error codes and
failed tasks in `veld eval --json` decide what gets worked on. Items are
roughly ordered.

## v0.2 — close gaps agents hit
- [ ] Tuples or anonymous records for multi-value `match` (see DESIGN.md
      trade-offs); write an RFC first.
- [ ] Record patterns: `case User{name, ..}`.
- [ ] `veld check --watch` and an LSP server (diagnostics, hover types, go to
      definition) for agent IDE integrations.
- [ ] Exact exhaustiveness for list patterns with elements after `..rest`.
- [ ] Explicit effect variables for higher-order functions stored in data.
- [ ] More evals (target 50 tasks) spanning CLI tools, data processing,
      HTTP services and algorithms; a script that runs a model against them.

## v0.3 — speed
- [ ] Compile to a bytecode VM (keep the tree-walker as the reference).
- [ ] Persistent data structures for List and Map (O(log n) updates).
- [ ] Tail calls.

## v0.4 — ecosystem
- [ ] Packages: `veld.json` dependencies fetched from git with lockfile.
- [ ] Std additions driven by evals: `std.regex`, `std.path`, `std.process`,
      `std.sql` (SQLite), `std.crypto` (hashing), `std.datetime`.
- [ ] Concurrency primitives with effect `async` (structured tasks, channels).

## v1.0 — "applications of all types"
- [ ] Native compilation (via Go or LLVM IR generation) and WebAssembly target.
- [ ] Foreign function interface declared with effects.
- [ ] Stability guarantee for syntax and diagnostic codes.

## Done
- v0.1: parser with recovery and fixes, unification-based checker, effects,
  exhaustiveness with witnesses, typed holes, contracts, interpreter, stdlib
  (str, list, map, math, option, result, json, io, fs, env, time, rand, http),
  formatter, test runner, `fix`, `describe`, `spec`, `explain`, `ast`, `eval`.
