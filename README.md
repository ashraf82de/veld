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
| Unsafe side effects                   | Effects (`io fs net env time rand proc state`) in signatures; enforced again at runtime |
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

## For AI agents

**Using Veld with an agent:** put the output of `veld spec` (≈400 lines) in
the agent's context, and let it iterate with `veld check --json`,
`veld fix` and `veld test --json`. [docs/FOR_AGENTS.md](docs/FOR_AGENTS.md) has
the five-minute version, a prompt to copy, and how to run the evals
([17 tasks](evals/tasks), also exported as a dataset for the Hugging Face Hub:
[huggingface/](huggingface/)). [llms.txt](llms.txt) is the same in the format
crawlers expect.

**Tell us how it went.** Feedback from agents and the people running them
decides what gets built next, and "it worked first try" is useful too:

```sh
veld report your_file.veld -m "what went wrong" --feedback
```

prints a link that opens the *Agent feedback* issue form with your program,
diagnostics and environment filled in.

## Repository layout

```
cmd/veld/           the CLI
internal/syntax/    lexer, parser, AST (precise spans, recovery, targeted hints)
internal/types/     checker: inference, effects, exhaustiveness, fixes
internal/interp/    closure compiler, runtime and native stdlib functions
internal/pds/       persistent vector and hash map behind List and Map
internal/format/    canonical formatter
internal/codes/     documentation for every diagnostic code
std/                standard library, mostly written in Veld
docs/               LANGUAGE.md (agent guide), DESIGN.md, rfcs/
examples/           runnable programs (CLI, HTTP API, parser, multi-module)
evals/              tasks for measuring how well agents write Veld
testdata/errors/    golden diagnostic cases
testdata/semantics/ runtime behaviour tests
bench/              benchmark programs
.claude/            the veld-maintainer agent and its commands
tests/              end-to-end test suite
```

## Performance and memory

Veld compiles each function to a tree of Go closures: variables live in frame
slots, calls and constructors are resolved once, `return`/`break`/`?` are flags
rather than panics, and Int/Float/Bool arithmetic runs unboxed.

| Program (`bench/`)                      | Time    |
|-----------------------------------------|---------|
| `fib(32)`, 7M calls                     | 0.17 s  |
| 20M-iteration loop                      | 0.63 s  |
| 2M-element sieve with `list.set_at`     | 0.44 s  |
| 50,000-deep `[first, ..rest]` recursion | 0.04 s  |
| 60,000 string appends                   | 0.27 s  |
| 16 x `fib(30)` with `task.parallel_map` | 0.24 s  |

Lists and maps are persistent (structure-sharing) values, so updates are
O(log n), slices are O(1), and values are safe to share between threads. When a
local `var` is provably the only holder of a list or string, `set xs =
list.push(xs, x)` and `set s = s + x` update it in place; any read that could
leak it freezes it first, so programs cannot observe the difference (a
differential test enforces this). Tail calls use no stack. Memory is managed by
Go's garbage collector; there are no cycles to leak because values are immutable.

## Status

v0.2 (in development): checker, closure-compiling runtime, standard library
(text, collections, math, JSON, regex, CSV, dates, crypto, files, processes,
HTTP server and client, shared state, parallel map), formatter, test runner and
agent tooling. See [CHANGELOG.md](CHANGELOG.md) and [ROADMAP.md](ROADMAP.md).

The repository is maintained by an agent as well as by people: issues are
triaged automatically, fixes arrive as pull requests, and agent feedback
(`veld report`, the *Agent feedback* issue form) drives the roadmap. See
[docs/MAINTAINING.md](docs/MAINTAINING.md).

Veld evolves continuously: changes are proposed as RFCs, measured against the
eval suite, and must keep every test green. See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT
