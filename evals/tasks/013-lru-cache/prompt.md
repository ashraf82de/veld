Write `solution.veld` implementing a least-recently-used cache of Int values:

    pub record Cache
      capacity: Int
      order: List[Str]          # keys, least recently used first
      values: Map[Str, Int]
    end record

    pub fn new(capacity: Int) -> Cache
    pub fn put(cache: Cache, key: Str, value: Int) -> Cache
    pub fn get(cache: Cache, key: Str) -> Pair[Option[Int], Cache]

`put` inserts or updates a key and makes it the most recently used one. When
that would exceed `capacity`, the least recently used key is evicted first.
`get` returns the value (`None` if absent) together with the cache in which that
key has become the most recently used. A capacity below 1 behaves like 1.
`Pair` is a prelude record with fields `first` and `second`.
