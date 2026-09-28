# RFC 0001: Matching on several values at once

- Status: implemented
- Author: veld-maintainer
- Date: 2026-09-29

## Problem

State machines, routers and comparisons of two options all need to match on a
combination of values. Veld has no tuples (DESIGN.md, "Known trade-offs"), so an
agent has three bad options:

1. `match [a, b]` builds a list, which only works when both values have the same
   type, and then fails to be exhaustive-checkable.
2. Nested `match` inside `match`, which doubles the nesting and repeats the
   fallback arm.
3. A throw-away record type just to match on it.

The same friction shows up in nearly every program that handles a `(state,
event)` pair, an HTTP `(method, path)` pair, or two `Option`s.

## Proposal

`match` accepts several comma-separated values, and each `case` lists one pattern
per value:

```
match state, event
  case Idle, Start => Running
  case Running, Stop => Idle
  case Running, Tick(n) if n > 10 => Timeout
  case s, _ => s
end match
```

- Every `case` must have exactly as many patterns as the `match` has values.
- Exhaustiveness, reachability warnings, guards and or-patterns work exactly as
  for a single value, across all columns (`case Idle, Start | Stop => ...`
  alternatives apply per column).
- The missing-case witness lists one pattern per value: ``missing case `Idle,
  Stop` ``, with the same machine-applicable fix.
- There is still no tuple *type* and no tuple *value*: the comma form exists only
  in `match` heads and `case` heads.

Implementation: the parser desugars `match a, b` to a match on a prelude
constructor (`Tuple2`, `Tuple3`, `Tuple4`, at most four values) and `case p, q`
to the corresponding constructor pattern, so type inference and the exhaustiveness
algorithm are unchanged. The interpreter avoids allocating the constructor by
matching the values directly. The formatter prints the comma form back.

## One way to do it

This adds a way to write a two-value match, but removes the need for the
workarounds above; those remain legal but are no longer the recommended form.
`match [a, b]` on same-typed values still works and is not deprecated.

## Diagnostics

- `E507` a `case` lists a different number of patterns than the `match` has
  values. The message says how many each has, and the fix rewrites the `case`
  head with `_` placeholders when it lists too few.
- `E508` more than four values in one `match` head.
- `E509` a `case` with commas in a single-value `match`; the note tells the agent
  to write `match a, b`.
- `E119` no longer mentions tuples for `match` heads; a tuple in any other
  position still gets the record hint.

## Measurement

`evals/tasks/009-state-machine` (a `(state, event)` transition function) and
`010-router` (a `(method, path)` router). Both are written naturally with the
new form; both need nested matches without it.

## Alternatives considered

- **First-class tuples.** A bigger language change (type syntax, construction,
  destructuring, printing). The design goal is fewer ways to write things, and
  the pain is concentrated in `match`; if agents later ask for tuples elsewhere
  a follow-up RFC can build on the same prelude constructors.
- **Nested matches only.** Status quo; produces the deepest code in the corpus.
- **Match on a list of `Option`s.** Requires equal types; does not scale.
