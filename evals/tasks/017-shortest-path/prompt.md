Write `solution.veld` with

    pub record Edge
      from: Str
      to: Str
      cost: Int
    end record

    pub fn shortest(edges: List[Edge], from: Str, to: Str) -> Option[Int]

`edges` describe a directed graph with non-negative costs. Return the cost of the
cheapest path from `from` to `to`, `Some(0)` when they are the same node, and
`None` when `to` is unreachable (or a node does not appear at all). The graph can
have thousands of edges: the solution must be much faster than trying every path.

Call it with named arguments, as Veld requires for three or more parameters:
`shortest(edges, from: "a", to: "b")`.
