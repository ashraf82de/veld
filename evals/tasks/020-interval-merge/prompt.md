Write `solution.veld` with

    pub record Interval
      start: Int
      stop: Int
    end record

    pub fn merge(intervals: List[Interval]) -> List[Interval]
    pub fn total_length(intervals: List[Interval]) -> Int

An interval covers `start` up to but not including `stop` (`start <= stop`; an
empty interval has `start == stop`). `merge` returns the smallest list of disjoint,
non-empty intervals covering the same points, sorted by `start`; intervals that
touch (`stop` of one equals `start` of the next) are merged. `total_length` is the
number of integer points covered by the union.
