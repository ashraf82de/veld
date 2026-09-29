# Show: Veld, a language designed for LLM agents to write (looking for agent feedback)

I have been building **Veld**, a statically typed language whose design goal is
"maximize the probability that code written by an AI agent is correct, and
minimize the iterations needed when it is not". It is MIT licensed and
pre-1.0: [github.com/ashraf82de/veld](https://github.com/ashraf82de/veld).

The failure modes it targets, and its answers:

| Failure of generated code | Veld |
|---|---|
| unbalanced braces, indentation drift | named closers (`end fn`, `end match`); errors say which block is open |
| many equivalent styles | one canonical layout, enforced by `veld fmt` |
| hallucinated APIs | no methods, a small fully listed stdlib, `veld describe`, did-you-mean fixes |
| null, exceptions, implicit coercion | `Option`, `Result` + `?`, no conversions |
| argument-order mistakes | calls with 3+ arguments must name them |
| "mutating" an immutable value | discarded results are a compile error with a `set x = ...` hint |
| unhandled cases | exhaustive `match` (also on several values) with a fix that inserts the case |
| unsafe side effects | effects in signatures, enforced again at runtime (`--deny`) |
| unverifiable output | `test` blocks and `requires`/`ensures` next to the code |
| prose error messages | `--json` diagnostics with stable codes, spans and fixes |

Runtime: closure-compiled interpreter over persistent lists/maps, tail calls,
in-place updates for provably unshared lists and strings, parallel map. Some
numbers from `bench/` on a laptop: fib(32) 0.17 s, 20M-iteration loop 0.63 s,
2M sieve 0.44 s. It is an interpreter, not a native compiler, and the syntax can
still change before 1.0.

There is a 17-task eval set with hidden tests (also on the Hugging Face Hub) so
you can measure how well a given model does with the guide plus compiler
feedback.

What I am asking for: run your agent on a task in Veld and tell me where it got
stuck. `veld report file.veld -m "what went wrong" --feedback` prints a link
that opens a pre-filled feedback issue. The repo is maintained partly by an
agent that triages issues and opens PRs, so reports get turned into eval tasks
and diagnostics fast. Critical takes on the design are just as welcome.
