Write `solution.veld` with

    pub fn wrap(text: Str, width: Int) -> List[Str]

Greedy word wrap: split `text` into words (runs of non-space characters) and pack
as many words as fit on each line, separated by single spaces, so that no line is
longer than `width` characters. A word longer than `width` gets a line of its own
(it is not split). Text without words gives `[]`.
