# Veld: a programming language for agents to write, and a small eval set to try it on

Most languages were designed for human keyboards: terse syntax, many ways to
say one thing, implicit conversions, error messages meant to be read by a
person. Those are exactly the places code-writing models fail: unbalanced
braces in long outputs, hallucinated methods, silent nulls, argument-order
mix-ups, "fixes" that mutate nothing, and diagnostics that describe a problem
without saying how to fix it.

**Veld** ([github.com/ashraf82de/veld](https://github.com/ashraf82de/veld)) inverts
those trade-offs. It is verbose where verbosity removes ambiguity and strict
where strictness turns a runtime surprise into a compile error:

- named block closers (`end fn`, `end match`), no significant indentation, one
  canonical layout enforced by `veld fmt`
- no null, no exceptions (`Option`, `Result` and `?`), no implicit conversions,
  no shadowing, immutable values
- calls with three or more arguments must name all but the first argument,
  reducing positional argument mix-ups
- effects (`io fs net env time rand proc state`) are part of every signature and
  enforced again at runtime; `veld run --deny net,fs file.veld` denies network
  and filesystem effects (other effects remain available)
- exhaustive `match`, including `match state, event`, with the missing case
  inserted for you
- a toolchain that speaks JSON: every diagnostic has a stable code, an exact
  span and usually a machine-applicable fix (`veld fix`); `veld spec` prints the
  whole ~400-line language guide for an agent's context

It is also meant to run fast and lean: functions compile to closures over
persistent (structure-sharing) lists and maps, tail calls use no stack, and
`set xs = list.push(xs, x)` on a local variable updates in place when nothing
else can see the old list (a differential test checks that this is never
observable). `fib(32)` takes about 0.2 s on a laptop; a 2-million-element sieve
about 0.4 s. It is a pre-1.0 interpreter, not a native compiler.

## Try it

```sh
go install github.com/ashraf82de/veld/cmd/veld@latest
veld spec            # the guide, for the model's context
veld new app && veld test app
```

There are 24 programming tasks with tests and reference solutions in the
[Veld evaluation suite](https://github.com/ashraf82de/veld/tree/main/evals/tasks)
(state machines, routers, CSV and JSON processing, regex, an LRU cache, matrix
math, shortest paths). Give a model the guide and a task prompt, let it use
`veld check --json` and `veld fix`, and grade with `veld eval`. The report
includes a histogram of the diagnostic codes the model triggered, which is the
signal we use to decide what to change. Keep the tests and reference solutions
out of the model's prompt when measuring its performance. A Hub-ready export
is in [huggingface/](https://github.com/ashraf82de/veld/tree/main/huggingface);
add a Hub dataset link only after publication has been verified.

## What we would like from you

If you point an agent at Veld, tell us how it went, including when it worked
first try. The fastest way:

```sh
veld report your_file.veld -m "what went wrong" --feedback
```

prints a link that opens the *Agent feedback* issue form with your program,
diagnostics and environment pre-filled. Reports help maintainers add regression
tests, improve diagnostics, and choose the next language improvements.

Useful details: the model, how many iterations it needed, which diagnostic was
confusing, which standard-library function you expected to exist.
