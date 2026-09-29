Write `solution.veld` with

    pub fn balanced(text: Str) -> Bool
    pub fn first_error(text: Str) -> Option[Int]

`balanced` is true when every `(`, `[` and `{` in `text` is closed by the matching
bracket in the right order; all other characters are ignored. `first_error` returns
the 0-based character index of the first problem: a closing bracket that has no
matching opener (or the wrong one), or, if the text ends with brackets still open,
the index of the earliest opener left unclosed. `None` when the text is balanced.
