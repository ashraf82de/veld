# RFC 0002: Record patterns

- Status: implemented
- Author: veld-maintainer
- Date: 2026-09-30

## Problem

Records can be built (`User{name: "a", age: 3}`) and updated (`User{..u, age: 4}`)
but not taken apart in a pattern. Agents write `case User{name, age} => ...` by
habit (Rust, Swift, JavaScript destructuring) and get E124 "record patterns are
not supported"; the workaround, binding the value and reading `u.name`, costs a
line per field and hides which fields a `case` depends on. Records that appear
inside sum types (`case Login(User{name, ..})`) cannot be matched at all
without a helper function.

## Proposal

```
match event
  case Login(User{name, admin: true, ..}) => "admin ${name}"
  case Login(User{name, ..}) => "user ${name}"
  case Logout(User{name: who, age: _}) => "bye ${who}"
end match
```

- `Type{field, field: pattern, ..}`; `field` alone binds a variable of that name.
- Every field must be listed, or the pattern must end with `..` (fields you do
  not care about). This keeps a `case` honest when a record gains a field.
- `module.Type{...}` works for records from other modules.
- A record has exactly one shape, so a record pattern is irrefutable unless a
  field pattern is; exhaustiveness and unreachable-case warnings work as for
  constructors, with the same witnesses.

## One way to do it

Adds a way to read fields (`u.name` remains the way to read one field of a value
you already have). It replaces the previous workaround of binding then
projecting, and removes the "not supported" error.

## Diagnostics

- `E510` a record pattern neither lists every field nor ends with `..`; the fix
  inserts `, ..`.
- `E511` an unknown record or field in a pattern (with did-you-mean), or a name
  that is a sum type.

## Measurement

Eval tasks that take records apart in `match` (024-date-diff style code, the
bank and LRU tasks) get shorter; add a golden case for each diagnostic.

## Alternatives considered

Positional patterns (`User("a", 3)`): fragile when fields are reordered and easy
to get wrong, the opposite of what named arguments are for.
