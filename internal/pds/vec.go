// Package pds provides the persistent (immutable, structure-sharing) data
// structures behind Veld's List and Map.
//
// Every operation returns a new value and leaves its receiver untouched, so
// values can be shared freely between goroutines. Updates cost O(log32 n)
// time and allocate only the path they change; the rest is shared. That is
// what makes `set xs = list.push(xs, x)` in a loop linear rather than
// quadratic, without giving up value semantics.
package pds

const (
	shiftBits = 5
	width     = 1 << shiftBits
	mask      = width - 1
)

type node struct {
	kids []*node // internal node: up to 32 children
	vals []any   // leaf node: up to 32 values
}

// Vec is an immutable vector: a 32-way trie plus a tail array that makes
// appends cheap.
type Vec struct {
	cnt   int
	shift uint
	root  *node
	tail  []any
}

var emptyVec = &Vec{shift: shiftBits, root: &node{}}

// Empty returns the empty vector.
func Empty() *Vec { return emptyVec }

// FromSlice builds a vector holding a copy of xs in O(n).
func FromSlice(xs []any) *Vec {
	n := len(xs)
	if n == 0 {
		return emptyVec
	}
	tailStart := ((n - 1) >> shiftBits) << shiftBits
	tail := make([]any, n-tailStart)
	copy(tail, xs[tailStart:])
	nodes := make([]*node, 0, tailStart>>shiftBits)
	for i := 0; i < tailStart; i += width {
		vals := make([]any, width)
		copy(vals, xs[i:i+width])
		nodes = append(nodes, &node{vals: vals})
	}
	shift := uint(shiftBits)
	for len(nodes) > width {
		parents := make([]*node, 0, (len(nodes)+width-1)/width)
		for i := 0; i < len(nodes); i += width {
			end := min(i+width, len(nodes))
			parents = append(parents, &node{kids: append([]*node(nil), nodes[i:end]...)})
		}
		nodes = parents
		shift += shiftBits
	}
	return &Vec{cnt: n, shift: shift, root: &node{kids: nodes}, tail: tail}
}

// Len is the number of elements.
func (v *Vec) Len() int { return v.cnt }

func (v *Vec) tailoff() int {
	if v.cnt < width {
		return 0
	}
	return ((v.cnt - 1) >> shiftBits) << shiftBits
}

func (v *Vec) arrayFor(i int) []any {
	if i >= v.tailoff() {
		return v.tail
	}
	n := v.root
	for level := v.shift; level > 0; level -= shiftBits {
		n = n.kids[(i>>level)&mask]
	}
	return n.vals
}

// Get returns element i. The caller must ensure 0 <= i < Len().
func (v *Vec) Get(i int) any { return v.arrayFor(i)[i&mask] }

// Set returns a vector with element i replaced. 0 <= i < Len().
func (v *Vec) Set(i int, x any) *Vec {
	if i >= v.tailoff() {
		tail := append([]any(nil), v.tail...)
		tail[i&mask] = x
		return &Vec{cnt: v.cnt, shift: v.shift, root: v.root, tail: tail}
	}
	return &Vec{cnt: v.cnt, shift: v.shift, root: assoc(v.shift, v.root, i, x), tail: v.tail}
}

func assoc(level uint, n *node, i int, x any) *node {
	if level == 0 {
		vals := append([]any(nil), n.vals...)
		vals[i&mask] = x
		return &node{vals: vals}
	}
	kids := append([]*node(nil), n.kids...)
	sub := (i >> level) & mask
	kids[sub] = assoc(level-shiftBits, kids[sub], i, x)
	return &node{kids: kids}
}

// Push returns a vector with x appended.
func (v *Vec) Push(x any) *Vec {
	if len(v.tail) < width {
		tail := make([]any, len(v.tail)+1, width)
		copy(tail, v.tail)
		tail[len(v.tail)] = x
		return &Vec{cnt: v.cnt + 1, shift: v.shift, root: v.root, tail: tail}
	}
	tailNode := &node{vals: v.tail}
	shift := v.shift
	var root *node
	if (v.cnt >> shiftBits) > (1 << v.shift) {
		root = &node{kids: []*node{v.root, newPath(v.shift, tailNode)}}
		shift += shiftBits
	} else {
		root = pushTail(v.cnt, v.shift, v.root, tailNode)
	}
	tail := make([]any, 1, width)
	tail[0] = x
	return &Vec{cnt: v.cnt + 1, shift: shift, root: root, tail: tail}
}

func newPath(level uint, n *node) *node {
	if level == 0 {
		return n
	}
	return &node{kids: []*node{newPath(level-shiftBits, n)}}
}

func pushTail(cnt int, level uint, parent, tailNode *node) *node {
	sub := ((cnt - 1) >> level) & mask
	kids := make([]*node, len(parent.kids), max(len(parent.kids)+1, sub+1))
	copy(kids, parent.kids)
	var ins *node
	if level == shiftBits {
		ins = tailNode
	} else if sub < len(kids) {
		ins = pushTail(cnt, level-shiftBits, kids[sub], tailNode)
	} else {
		ins = newPath(level-shiftBits, tailNode)
	}
	if sub < len(kids) {
		kids[sub] = ins
	} else {
		kids = append(kids, ins)
	}
	return &node{kids: kids}
}

// Range calls f for each element in order until f returns false.
func (v *Vec) Range(f func(i int, x any) bool) {
	off := 0
	stop := false
	var walk func(level uint, n *node)
	walk = func(level uint, n *node) {
		if level == 0 {
			for _, x := range n.vals {
				if !f(off, x) {
					stop = true
					return
				}
				off++
			}
			return
		}
		for _, k := range n.kids {
			walk(level-shiftBits, k)
			if stop {
				return
			}
		}
	}
	if v.tailoff() > 0 {
		walk(v.shift, v.root)
		if stop {
			return
		}
	}
	for _, x := range v.tail {
		if !f(off, x) {
			return
		}
		off++
	}
}

// ToSlice copies the elements into a new slice.
func (v *Vec) ToSlice() []any {
	out := make([]any, 0, v.cnt)
	v.Range(func(_ int, x any) bool {
		out = append(out, x)
		return true
	})
	return out
}

// Slice returns elements [start, stop) as a new vector. Bounds must satisfy
// 0 <= start <= stop <= Len().
func (v *Vec) Slice(start, stop int) *Vec {
	if start == 0 && stop == v.cnt {
		return v
	}
	out := make([]any, 0, stop-start)
	v.Range(func(i int, x any) bool {
		if i >= stop {
			return false
		}
		if i >= start {
			out = append(out, x)
		}
		return true
	})
	return FromSlice(out)
}

// Concat returns the elements of v followed by those of w.
func (v *Vec) Concat(w *Vec) *Vec {
	if w.cnt == 0 {
		return v
	}
	if v.cnt == 0 {
		return w
	}
	if w.cnt > v.cnt {
		return FromSlice(append(v.ToSlice(), w.ToSlice()...))
	}
	out := v
	w.Range(func(_ int, x any) bool {
		out = out.Push(x)
		return true
	})
	return out
}
