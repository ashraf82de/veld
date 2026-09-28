Write `solution.veld` defining:

    pub fn parse_pairs(text: Str) -> Result[Map[Str, Int], Str]

`text` holds `key=value` pairs separated by `;`, e.g. `"a=1;b=22"`. Surrounding
whitespace around keys and values is ignored; empty segments are skipped.
Return `Err("bad pair: <segment>")` for a segment without exactly one `=`, and
`Err("bad number: <value>")` when the value is not an integer.
