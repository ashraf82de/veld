# Veld design

Veld's single design goal: **maximize the probability that code written by an
AI agent is correct, and minimize the number of iterations needed when it is
not.** Human ergonomics (brevity, cleverness, familiarity for its own sake) is
not a goal. Where the two conflict, the agent wins.

This document records each decision and the failure mode it targets, so that
future changes can be judged against the same yardstick.

## 1. Text layout the model cannot lose track of

- **Named block closers** (`end fn`, `end if`, `end match`). A model
  generating a 300-line file must track nesting; `}` carries no information
  about what it closes, so errors surface far from their cause. With named
  closers the parser knows which block is open and reports "found `end fn`
  but the innermost open block is the `if` on line 12", with a fix.
- **Indentation is not significant.** Whitespace-sensitive languages turn
  copy/paste and re-indentation mistakes into semantic changes.
- **Newlines end statements; no semicolons.** Continuation rules are purely
  lexical (trailing operator or leading `|>`/`.`), so they are easy to state.
- **Canonical formatting.** `veld fmt` is idempotent and there is exactly one
  layout per syntax tree. Canonical code is easier to diff, patch and learn.

## 2. One way to do each thing

Every construct exists once: no `while` + `loop` + `do-while`; no ternary
next to `if`; no methods next to functions; no exceptions next to Result;
no `x += 1` next to `set x = x + 1`. Fewer choices mean fewer plausible
wrong choices and more consistent training signal as the corpus grows.

## 3. Nothing implicit

- No `null`: `Option[T]`. No exceptions: `Result[T, E]` with `?`.
- No implicit numeric conversion (`Int + Float` is an error with a note).
- No truthiness: conditions are `Bool`.
- No shadowing: a name means one thing within a function.
- No global mutable state: modules contain only declarations. This also
  makes concurrent HTTP handlers safe by construction.
- Values are immutable; only `var` bindings can be reassigned, with the
  greppable keyword `set`.

## 4. Local reasoning

- Every function signature is fully typed (parameters, return type,
  effects). A model reading one function never needs another function's body.
- Locals are inferred (unification), because annotating them adds tokens
  without adding information.
- **Effects** (`io fs net env time rand`) are part of the signature and
  propagate to callers. An agent (or a reviewer of agent code) can see from
  the signature alone whether a function touches the network. Higher-order
  functions are effect-polymorphic in their function parameters. The runtime
  enforces the same capabilities, so `veld run --deny net` is a real sandbox
  even if static checking is bypassed.

## 5. Mistakes models actually make become compile errors

- **Argument order**: calls with 3+ arguments must name all but the first.
- **Discarded results**: `list.push(xs, 1)` as a statement is an error
  (E308) with the note that values are immutable, because models trained on
  mutable collections write exactly that.
- **Non-exhaustive matches** report a concrete missing pattern and insert it.
- **Unknown names** get did-you-mean fixes, and names from other languages
  (`len`, `print`, `append`, `null`, `String`, `&&`) get the Veld equivalent.
- **"Methods"** like `xs.len()` get "Veld has no methods; call list.len(xs)".

## 6. The toolchain is an API

- Every diagnostic has a stable code (`veld explain CODE`), an exact span,
  notes, and zero or more fixes expressed as text edits.
- `veld fix` applies the fixes; the test suite checks that fixes actually
  repair the errors they target.
- `--json` everywhere. Runtime errors carry a Veld stack trace and, for
  failed comparisons in `expect`/contracts, both operand values.
- **Typed holes** (`???`) let an agent write a skeleton first and ask the
  checker what each gap needs.
- `veld spec` prints the whole language guide, so the tool is self-describing
  and an agent never needs network access to learn it.
- Cascading errors are suppressed (poisoned types) so the first error is the
  one that matters.

## 7. Verification lives with the code

`test` blocks sit next to the functions they test; `requires`/`ensures`
contracts are checked at runtime with operand values in the report. Agents can
write the specification and the implementation in one place and run both.

## 8. A small, closed standard library

The stdlib is small enough to list in the guide and is mostly written in Veld
itself (externs are implemented in `internal/interp/natives.go`). There is no
dependency on training-data familiarity with a large API surface.

## Known trade-offs

- **No tuples.** Records are clearer but matching two values at once is
  clumsier; evals show models reach for `match [a, b]` with mixed types.
  Candidate for an RFC.
- **Named arguments add tokens** for 3+ argument calls. Judged worth it.
- **Or-pattern/suffix-pattern exhaustiveness** is conservative: list patterns
  with elements after `..rest` count as covering nothing.
- **Effect polymorphism** is limited to function-typed parameters without an
  explicit `uses`; storing an effectful closure in a data structure can hide
  its effect from the checker (the runtime still enforces capabilities).
- **Performance**: v0.1 is a tree-walking interpreter with copy-on-update
  collections. Correctness first; see the roadmap.

## How decisions are evaluated

A language change should improve at least one of: first-try compile rate,
first-try test pass rate, or iterations-to-green on `evals/`, without
regressing the others. `veld eval --json` reports per-task results and a
count of error codes, which is the main signal for what to change next.
