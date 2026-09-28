Write `solution.veld` with

    pub fn totals(text: Str) -> Result[List[Str], Str]

`text` is CSV with a header line `category,amount` followed by data rows, for
example `food,12`. Sum the amounts per category and return one line per
category, formatted `category: total`, sorted by category name.

- Fields may be quoted (`"office, misc",5`).
- A row that does not have exactly two fields, or whose amount is not an
  integer, makes the whole result `Err("bad row N")`, where N is the line
  number of the row counting the header as line 1.
- A missing or different header is `Err("bad header")`.
- Blank lines are ignored (and do not count as rows).
