# Veld

**A programming language designed for AI agents to write, not for humans to type.**

Mainstream languages were optimized for human keyboards and human memory:
terse syntax, many ways to say the same thing, implicit conversions, and error
messages meant to be read by a person. Those same properties are where code-
generating models fail most: mismatched braces in long outputs, hallucinated
methods, silent `null`s, argument order mix-ups, "fixed" code that mutates
nothing, and compiler errors that describe a problem without saying how to
fix it.

Veld inverts the trade-offs. It is verbose where verbosity removes ambiguity,
strict where strictness turns a runtime surprise into a compile error, and its
toolchain speaks JSON with exact spans and machine-applicable fixes.

```veld
use std.io
use std.list

type Shape
  case Circle(radius: Float)
  case Rect(width: Float, height: Float)
end type

fn area(s: Shape) -> Float
  match s
    case Circle(r) => 3.14159 * r * r
    case Rect(w, h) => w * h
  end match
end fn

fn main() -> Unit uses io
  let total = [Circle(1.0), Rect(width: 2.0, height: 3.0)]
    |> list.map(area)
    |> list.sum_float()
  io.print("total area: ${total}")
end fn

test "rectangle"
  expect area(Rect(width: 2.0, height: 3.0)) == 6.0
end test
```

## What makes it agent-first

| Failure mode of generated code        | Veld's answer                                                          |
|---------------------------------------|------------------------------------------------------------------------|
| Unbalanced braces / indentation drift | Named block closers (`end fn`, `end match`); errors say which block is open |
| Many equivalent styles                | One canonical form, enforced by `veld fmt`; one way to write each construct |
| Hallucinated APIs and methods         | No methods; small, fully listed stdlib; `veld describe`; did-you-mean fixes |
| `null`, exceptions, implicit coercion | `Option`, `Result` + `?`, no implicit conversions                       |
| Argument order mistakes               | Calls with 3+ arguments must name them                                  |
| "Mutating" immutable values           | Discarded results are a compile error with a `set x = ...` hint         |
| Unhandled cases                       | Exhaustive `match`, with the missing case and a fix that inserts it     |
| Unsafe side effects                   | Effects (`io fs net env time rand`) in signatures; enforced again at runtime |
| Guessing what to write next           | Typed holes: `???` reports the needed type and variables in scope       |
| Unverifiable output                   | Tests and `requires`/`ensures` contracts live next to the code          |
| Prose error messages                  | `--json` diagnostics: stable codes, spans, fixes; `veld fix` applies them |

The full rationale is in [docs/DESIGN.md](docs/DESIGN.md).

## Install

Requires Go 1.22+.

```sh
go install github.com/ashraf82de/veld/cmd/veld@latest   # or: go build -o veld ./cmd/veld
```

## Use

```sh
veld new hello && veld run hello/main.veld
veld check app --json        # diagnostics as JSON
veld fix app/main.veld       # apply suggested fixes
veld test app --json         # run test blocks
veld fmt app                 # canonical formatting
veld describe std.list       # signatures and docs
veld spec                    # the complete language guide, for an agent's context
veld explain E303            # explain a diagnostic code
veld eval evals/tasks sols/  # grade agent-written solutions
```

**Using Veld with an agent:** put the output of `veld spec` (≈400 lines) in
the agent's context, and let it iterate with `veld check --json`,
`veld fix` and `veld test --json`.

## Repository layout

```
cmd/veld/           the CLI
internal/syntax/    lexer, parser, AST (precise spans, recovery, targeted hints)
internal/types/     checker: inference, effects, exhaustiveness, fixes
internal/interp/    reference interpreter and native stdlib functions
internal/format/    canonical formatter
internal/codes/     documentation for every diagnostic code
std/                standard library, mostly written in Veld
docs/               LANGUAGE.md (agent guide), DESIGN.md, rfcs/
examples/           runnable programs (CLI, HTTP API, parser, multi-module)
evals/              tasks for measuring how well agents write Veld
testdata/errors/    golden diagnostic cases
tests/              end-to-end test suite
```

## Status

v0.1: a complete tree-walking implementation with a static checker, stdlib
(strings, lists, maps, math, JSON, files, env, time, random, HTTP server and
client), formatter, test runner and agent tooling. See [ROADMAP.md](ROADMAP.md)
for what comes next (compiled backend, effect polymorphism, packages, ...).

Veld evolves continuously: changes are proposed as RFCs, measured against the
eval suite, and must keep every test green. See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT
