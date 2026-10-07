Write `solution.veld` defining:

    pub record Renewal
      name: Str
      owner: Str
      category: Str
      expires_day: Int
    end record

    pub fn due(
      items: List[Renewal],
      today: Int,
      within: Int,
      owner: Str,
      category: Str,
    ) -> Result[List[Str], Str]

Return renewals whose `expires_day - today` is at most `within`, including
overdue renewals. An empty owner or category filter matches every value;
otherwise compare that field after trimming whitespace and ignoring case. Both
non-empty filters must match. A negative `within` returns
`Err("within must be zero or greater")`.

Sort results by `expires_day`, then by name on a tie. Format each line as:

    <status> | <name> | owner=<owner> | category=<category>

Status is `OVERDUE Nd`, `DUE TODAY`, or `DUE Nd`. Preserve the stored text in
the output; normalization is only for matching.
