Write `solution.veld` with

    pub fn days_between(from: Str, to: Str) -> Result[Int, Str]
    pub fn add_days(date: Str, days: Int) -> Result[Str, Str]
    pub fn weekday_name(date: Str) -> Result[Str, Str]

Dates are `YYYY-MM-DD` strings in UTC. `days_between` is the number of whole days
from `from` to `to` (negative if `to` is earlier). `add_days` returns the date that
many days later (or earlier for a negative count), formatted `YYYY-MM-DD`.
`weekday_name` is `Monday` ... `Sunday`. Any argument that is not a valid date gives
`Err("invalid date: <text>")` (check `from` before `to`). The standard library
module `std.datetime` helps; call `days_between(a, to: b)` with the second argument
named only if you use three or more parameters (here two is fine).
