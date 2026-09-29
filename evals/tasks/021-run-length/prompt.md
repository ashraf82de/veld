Write `solution.veld` with

    pub fn encode(text: Str) -> Str
    pub fn decode(text: Str) -> Result[Str, Str]

Run-length encoding of characters: `encode("aaabcc")` is `"3a1b2c"` (every run is
written as its length followed by the character; `""` encodes to `""`). Characters
are Unicode characters, not bytes. `decode` inverts `encode`: a count of one or
more digits followed by exactly one character, repeated. Malformed input (a
count without a character, a character without a count, or a count of `0`)
returns `Err("malformed input")`. `decode("")` is `Ok("")`.
