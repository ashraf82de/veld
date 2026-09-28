# Veld language guide (for AI agents)

Veld is a statically typed language designed to be written by AI agents.
This guide is the complete reference; `veld spec` prints it. Read it once,
then rely on `veld check --json` — every diagnostic has a stable code, an
exact span, and usually a fix you can apply (`veld fix`).

## The workflow

```
veld new app            # starter project
veld check app --json   # type/effect check; fix errors
veld fix app/main.veld  # apply the suggested fix of every error
veld test app --json    # run `test` blocks
veld run app/main.veld  # run main()
veld fmt app            # canonical formatting (one layout per program)
veld describe std.list  # signatures + docs of a module; `veld describe` lists modules
veld explain E303       # what a diagnostic code means
```

Write `???` for any expression you have not decided yet: the checker reports
the type it needs and the variables in scope (H001).

## Rules that differ from mainstream languages

1. Blocks close with `end` + the opening keyword: `end fn`, `end if`,
   `end match`, `end for`, `end while`, `end type`, `end record`, `end test`.
   Indentation is not significant. No braces, no semicolons, no colons after
   headers. Block bodies start on the next line.
2. No null, no exceptions, no implicit conversions, no methods, no classes,
   no inheritance, no global variables, no shadowing, no tuples, no indexing
   operator. Use Option, Result, explicit conversion functions, module
   functions, records/sum types, and `list.get`.
3. Every function signature is fully typed, including `-> Unit`.
4. Functions declare effects with `uses`: `io`, `fs`, `net`, `env`, `time`,
   `rand`, `proc`, `state`. Pure functions declare none. A caller must declare
   every effect of everything it calls.
5. Calls with 3 or more arguments name every argument after the first:
   `str.replace(s, old: "a", new: "b")`.
6. Values are immutable. `list.push(xs, 1)` returns a new list; discarding a
   non-Unit value is an error (E308). Reassign with `set xs = list.push(xs, 1)`.
7. `match` must be exhaustive.
8. Types and constructors are `PascalCase`; everything else is `snake_case`.

## Files, modules, imports

A file is a module. `use` lines come first.

```
use std.io          # standard library module, accessed as io.print(...)
use lib.text        # local module lib/text.veld, accessed as text.name(...)
```

Local paths resolve from the project root: the nearest directory containing
`veld.json`, or else the entry file's directory. Only `pub` declarations are
visible to other modules. The prelude (Option, Result, Json, panic) is always
in scope.

## Declarations

```
## Doc comment (## lines directly above a declaration).
pub fn area(width: Float, height: Float) -> Float
  width * height                        # last line of a block is its value
end fn

fn main() -> Unit uses io, fs           # effects after `uses`
  io.print("hi")
end fn

fn first[T](items: List[T]) -> Option[T]   # generics in square brackets
  list.get(items, 0)
end fn

fn divide(a: Int, b: Int) -> Int
  requires b != 0                       # contracts, checked at runtime
  ensures result * b <= a
  a / b
end fn

record User                             # product type, one field per line
  name: Str
  age: Int
end record

type Shape                              # sum type
  case Circle(radius: Float)
  case Rect(width: Float, height: Float)
  case Empty
end type

test "area of a rectangle"              # tests live next to the code
  expect area(2.0, 3.0) == 6.0
end test
```

`main` takes no parameters and returns `Unit` or `Result[Unit, E]` (an `Err`
prints the error and exits with status 1).

## Types

`Int` (64-bit, overflow is a runtime error), `Float`, `Bool`, `Str` (UTF-8,
lengths count characters), `Unit` (its value is written `Unit`), `List[T]`,
`Map[K, V]` (insertion-ordered), `Option[T]` (`Some(x)`, `None`),
`Result[T, E]` (`Ok(x)`, `Err(e)`), `Json`, `Fn(A, B) -> R`,
`Fn(A) -> R uses io`, user records and sum types, `module.Type`.

Local variable types are inferred. Annotate when inference needs help:
`let names: List[Str] = []`, `var counts: Map[Str, Int] = {}`.

## Statements

```
let x = 1                      # immutable binding
var total = 0                  # mutable binding
set total = total + x          # reassign a var
expect total == 1              # only inside test blocks
```

## Expressions

```
1   -7   1_000   3.14   1.0e-3   true   "text"   Unit
"interpolation: ${user.name} is ${user.age}"     # any value; \$ for a literal $
"""raw multi-line text: no escapes, no interpolation"""
[1, 2, 3]            {"a": 1, "b": 2}            # list, map literals
User{name: "a", age: 3}                          # record literal (all fields)
User{..u, age: 4}                                # copy with changes
Circle(1.0)   Rect(width: 1.0, height: 2.0)   Empty    # constructors
u.name                                           # field access
f(x)   f(x, y)   f(x, name: y, other: z)         # calls
xs |> list.map(fn(x) => x * 2)                   # pipe: first argument
fn(x) => x + 1                                   # lambda
fn(x: Int) -> Int                                # multi-line lambda
  let y = x * 2
  y + 1
end fn
a + b   a - b   a * b   a / b   a % b            # Int/Float (+ also Str, List)
a == b  a != b  a < b  a <= b  a > b  a >= b     # comparisons don't chain
a and b   a or b   not a                         # mixing and/or needs parens
value?                                           # unwrap Ok/Some or return Err/None
???                                              # typed hole
```

Int division truncates toward zero. `and`/`or` short-circuit. Operator
precedence, loosest first: `and`/`or`, `not`, comparisons, `|>`, `+ -`,
`* / %`, unary `-`, postfix (call, `.field`, `?`).

Line breaks: a line continues when it ends with an operator, `,`, `(`, `[`,
`{`, `=` or `=>`, or when the next line starts with `|>` or `.`. Inside
brackets, newlines around items are free (trailing commas allowed).

## Control flow

All of these are expressions; their value is the last line of the chosen
block.

```
let size = if n > 100
  "big"
else if n > 10
  "medium"
else
  "small"
end if

let label = match shape
  case Circle(r) => "circle ${r}"
  case Rect(w, h) if w == h => "square"
  case Rect(_, _) => "rectangle"
  case Empty => "nothing"
end match

for item in items              # iterate a List; use list.range(0, n) for numbers
  if item < 0
    continue
  end if
  set total = total + item
end for

while total > 0
  set total = total - 1
end while
```

`return value` exits the function (inside a lambda: the lambda). `break` and
`continue` work in loops.

Match arms are `case pattern => expression`, `case pattern => set x = ...`,
or `case pattern` followed by an indented block. Patterns:

```
_                     # anything
name                  # bind
42   -1   "text"   true                  # literals
Some(x)   Rect(w, _)   shapes.Circle(r)  # constructors (all fields)
[]   [a]   [a, b]   [first, ..rest]   [x, .._]    # lists
"+" | "-"   Some(1 | 2)                  # alternatives (cannot bind names)
case x if x > 0 => ...                   # guard
```

Match on several values at once (2 to 4) with commas; every `case` lists one
pattern per value, `|` alternatives apply to one value, and a lone `_` covers
every value:

```
match state, event
  case Idle, Start => Running
  case Running, Stop | Timeout => Idle
  case _ => state
end match
```

## Errors

Expected failures are values: return `Result[T, E]` and propagate with `?`.
`?` needs the enclosing function to return a `Result` with the same error type
(or an `Option` for Option values). Convert with `result.map_err` and
`option.ok_or`. `panic("message")` is only for impossible states.

```
fn load(path: Str) -> Result[Config, Str] uses fs
  let text = fs.read(path)?
  let n = str.to_int(text) |> option.ok_or("not a number")?
  Ok(Config{limit: n})
end fn
```

## Standard library

Import with `use std.<name>`. `veld describe std.<name>` lists everything.

- `io`: print, write, eprint, read_line, read_all (uses io)
- `str`: len, split, join, lines, words, trim, upper, lower, contains,
  starts_with, ends_with, replace, slice, index_of, count, to_int, to_float,
  chars, code, from_code, repeat, reverse, pad_left, pad_right, fixed,
  is_empty, at, is_digit, is_alpha, is_alnum, is_space, is_upper, is_lower,
  strip_prefix, strip_suffix, split_once
- `list`: len, is_empty, get, get_or, first, last, push, prepend, set_at,
  remove_at, range, map, map_indexed, filter, fold, flat_map, flatten, find,
  find_index, any, all, count, contains, index_of, sort, sort_by, sort_desc,
  sort_by_desc, reverse, take, drop, take_while, drop_while, slice, unique,
  repeat, sum, sum_float, max, min, max_by, min_by, chunks, zip, enumerate,
  partition, group_by
- `map`: len, is_empty, get, get_or, has, put, remove, keys, values, merge,
  update, entries, from_entries, map_values, filter
- `math`: abs, abs_float, min, max, min_float, max_float, clamp, sign, gcd,
  to_float, floor, ceil, round, trunc, sqrt, pow, pow_int, log, log10, exp,
  sin, cos, tan, asin, acos, atan, atan2, is_nan, pi
- `option`: is_some, is_none, unwrap_or, map, and_then, ok_or
- `result`: is_ok, is_err, unwrap_or, map, map_err, and_then, ok
- `json`: parse, encode, pretty, get, at, as_str, as_num, as_int, as_bool,
  as_list, object, string, int, number, boolean, array (values are the
  prelude type `Json`: JNull, JBool, JNum, JStr, JArr, JObj)
- `regex`: is_match, find, find_all, captures, replace, split. Patterns are
  RE2; write them as raw strings `"""\d+"""`. Every function returns a Result.
- `path`: join, base, dir, ext, stem, clean, is_absolute (text only)
- `encoding`: base64_encode/decode, hex_encode/decode, url_encode/decode
- `crypto`: sha256, sha512, hmac_sha256, constant_time_equal, random_hex and
  uuid (uses rand)
- `csv`: parse, encode
- `datetime`: iso, parse_iso, parts, from_parts, weekday (pure; timestamps
  are milliseconds since the epoch, UTC)
- `fs` (uses fs): read, write, append, exists, list_dir, make_dir, remove
- `env` (uses env): args, get, exit
- `time` (uses time): now_ms, sleep_ms
- `rand` (uses rand): int, float, shuffle
- `process` (uses proc): run, run_with_input; record
  `process.Output{status, stdout, stderr}`. No shell is involved.
- `state` (uses state): get, put, remove, keys, update, incr, save, load.
  Shared thread-safe storage of Json values, for servers that must remember
  things between requests (there are no global variables).
- `http` (uses net): serve, get, post, request, text, html, json_response,
  error, redirect, segments, query_param, header, json_body; records
  `http.Request{method, path, query, headers, body}` and
  `http.Response{status, headers, body}`

The prelude also has `Pair[A, B]{first, second}`, returned by `list.zip`,
`list.enumerate`, `list.partition`, `str.split_once` and `map.entries`.

## Performance and memory

Lists and maps are persistent: updates cost O(log n) and share structure with
the old value, so building a list with `set xs = list.push(xs, x)` in a loop
is linear. Prefer these idioms:

- `for i in list.range(a, b)` counts without building a list.
- `set xs = list.set_at(xs, index: i, item: v)` on a local `var` updates in
  place when nothing else can see the old list.
- Recursion over `[first, ..rest]` is cheap (`rest` is a view, not a copy),
  and calls nest up to 100,000 deep.
- Build big strings with `str.join(parts, "")`, not repeated `+`.
- `veld run --deny ...` caps what a program may do; there is no other resource
  limit beyond the call depth.

Common calls:

```
list.fold(xs, init: 0, step: fn(acc, x) => acc + x)
map.put(m, key: k, value: v)
map.get_or(m, key: k, default: 0)
map.update(m, key: k, default: 0, f: fn(n) => n + 1)
str.slice(s, start: 0, stop: 3)
str.pad_left(s, width: 5, fill: "0")
http.serve(8080, fn(req) => http.text(200, "hello"))
```

A tiny router, matching on the path segments:

```
match http.segments(req.path)
  case [] => http.text(200, "home")
  case ["todos"] => list_todos()
  case ["todos", id] => show_todo(id)
  case _ => http.error(404, "not found")
end match
```

## Translating habits from other languages

| You might write                 | In Veld                                     |
|---------------------------------|---------------------------------------------|
| `xs[i]`                         | `list.get(xs, i)` (returns Option)          |
| `xs.length`, `len(xs)`          | `list.len(xs)` or `xs |> list.len()`        |
| `xs.append(x)`                  | `set xs = list.push(xs, x)`                 |
| `d[k] = v`                      | `set d = map.put(d, key: k, value: v)`      |
| `str(x)`, `x.toString()`        | `"${x}"`                                    |
| `int(s)`                        | `str.to_int(s)` (returns Option)            |
| `null` / `None` check           | `Option` + `match` or `?`                   |
| `throw` / `raise`               | `return Err(...)`                           |
| `x += 1`                        | `set x = x + 1`                             |
| `a && b`, `a \|\| b`, `!a`      | `a and b`, `a or b`, `not a`                |
| `for (i = 0; i < n; i++)`       | `for i in list.range(0, n)`                 |
| `for k, v in d.items()`         | `for k in map.keys(d)` + `map.get`          |
| class with methods              | `record` + functions taking it              |
| enum / tagged union             | `type` with `case` variants                 |
| `if x: ...` on a non-Bool       | explicit comparison: `if n != 0`            |
| `print(...)`                    | `use std.io` + `io.print(...)` + `uses io`  |

## Complete example

```
use std.io
use std.list
use std.str

type Command
  case Add(amount: Int)
  case Reset
end type

fn parse(line: Str) -> Result[Command, Str]
  match str.words(line)
    case ["add", n]
      let amount = str.to_int(n) |> option_or_err("not a number: ${n}")?
      Ok(Add(amount))
    case ["reset"] => Ok(Reset)
    case _ => Err("unknown command: ${line}")
  end match
end fn

fn option_or_err(value: Option[Int], message: Str) -> Result[Int, Str]
  match value
    case Some(v) => Ok(v)
    case None => Err(message)
  end match
end fn

fn apply(total: Int, cmd: Command) -> Int
  match cmd
    case Add(n) => total + n
    case Reset => 0
  end match
end fn

fn main() -> Result[Unit, Str] uses io
  var total = 0
  for line in ["add 2", "add 40", "reset", "add 7"]
    set total = apply(total, parse(line)?)
  end for
  io.print("total: ${total}")
  Ok(Unit)
end fn

test "commands"
  expect parse("add 3") == Ok(Add(3))
  expect parse("jump") == Err("unknown command: jump")
  expect list.fold([Add(1), Add(2)], init: 0, step: apply) == 3
end test
```
