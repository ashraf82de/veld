Write `solution.veld` with

    pub fn emails(text: Str) -> List[Str]
    pub fn is_iso_date(text: Str) -> Bool
    pub fn redact_digits(text: Str) -> Str

- `emails` returns every email address in the text, in order. An address is
  letters, digits, `.`, `_` or `-`, then `@`, then a domain of labels made of
  letters, digits and `-`, separated by dots, ending in a dot and at least two
  letters (`bob.smith@mail.example.com`).
- `is_iso_date` is true for exactly `YYYY-MM-DD` with a month 01-12 and a day
  01-31 (it does not need to know month lengths).
- `redact_digits` replaces every run of digits with a single `#`.
