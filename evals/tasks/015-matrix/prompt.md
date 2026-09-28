Write `solution.veld` with

    pub fn transpose(m: List[List[Int]]) -> List[List[Int]]
    pub fn multiply(a: List[List[Int]], b: List[List[Int]]) -> Result[List[List[Int]], Str]
    pub fn identity(n: Int) -> List[List[Int]]

Matrices are lists of rows. `transpose` swaps rows and columns (`[]` stays `[]`;
all rows have equal length). `multiply` is the matrix product of an m x n and an
n x p matrix; if the inner dimensions differ it returns `Err("dimension mismatch")`.
`identity(n)` is the n x n identity matrix (`[]` for n <= 0).
