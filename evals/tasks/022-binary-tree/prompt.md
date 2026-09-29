Write `solution.veld` implementing a binary search tree of Ints:

    pub type Tree
      case Leaf
      case Node(left: Tree, value: Int, right: Tree)
    end type

    pub fn insert(tree: Tree, value: Int) -> Tree
    pub fn contains(tree: Tree, value: Int) -> Bool
    pub fn to_list(tree: Tree) -> List[Int]
    pub fn height(tree: Tree) -> Int
    pub fn from_list(values: List[Int]) -> Tree

`insert` keeps the tree ordered and ignores duplicates. `to_list` returns the
values in ascending order. `height` is 0 for `Leaf` and 1 + the larger child height
otherwise. `from_list` inserts the values in order. Call constructors with names:
`Node(left: l, value: v, right: r)`.
