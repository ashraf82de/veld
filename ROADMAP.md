# Roadmap

Priorities are set by feedback and the eval suite: the most frequent error
codes and failed tasks in `veld eval --json`, and issues labelled
`agent-feedback`, decide what gets worked on. Items are roughly ordered. The
`veld-maintainer` agent keeps this file honest (see `docs/MAINTAINING.md`).

## Next: close gaps agents hit
- [ ] Tail calls, so loops written as recursion never hit the depth limit.
- [ ] Map updates in place under the same ownership rule as lists
      (`set m = map.put(m, ...)` on a local `var`).
- [ ] Record patterns: `case User{name, ..}`.
- [ ] `veld check --watch` and an LSP server (diagnostics, hover types, go to
      definition) for agent IDE integrations.
- [ ] Exact exhaustiveness for list patterns with elements after `..rest`.
- [ ] Explicit effect variables for higher-order functions stored in data.
- [ ] More evals (17 today; target 50) spanning CLI tools, data processing,
      HTTP services and algorithms; a script that runs a model against them.
- [ ] `std.task`: structured concurrency (spawn, await, parallel map) with
      captured `var`s rejected at check time.
- [ ] `std.sql`: an embedded relational store (pure Go, no dependencies).

## Speed and memory
- [ ] Bytecode or register VM behind the closure compiler for tight numeric
      loops (the closure compiler stays the reference).
- [ ] Unboxed Int/Float slots in frames.
- [ ] String builders with the same ownership rule as lists, so `s = s + x` in
      a loop is linear.
- [ ] Resource limits: `veld run --max-memory`, `--max-steps`.

## Ecosystem
- [ ] Packages: `veld.json` dependencies fetched from git with lockfile.
- [ ] Foreign function interface declared with effects.

## v1.0: "applications of all types"
- [ ] Native compilation (via Go or LLVM IR generation) and WebAssembly target.
- [ ] Stability guarantee for syntax and diagnostic codes.

## Done
- v0.2 (in progress, see CHANGELOG.md): closure-compiling runtime with slot
  variables and unboxed arithmetic; persistent List/Map (`internal/pds`) with
  O(1) slices and in-place updates for provably unshared lists; call depth
  100k; multi-value `match a, b` (RFC 0001); effects `proc` and `state`;
  stdlib: regex, path, encoding, crypto, csv, datetime, process, state, plus
  additions to str, list, map, math, json, http; `veld report`; maintainer
  agent, issue forms, CI on Linux and Windows, release workflow.
- v0.1: parser with recovery and fixes, unification-based checker, effects,
  exhaustiveness with witnesses, typed holes, contracts, interpreter, stdlib
  (str, list, map, math, option, result, json, io, fs, env, time, rand, http),
  formatter, test runner, `fix`, `describe`, `spec`, `explain`, `ast`, `eval`.
