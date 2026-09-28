// Package pds provides the persistent (immutable, structure-sharing) data
// structures behind Veld's List and Map.
//
// Every operation returns a new value and leaves its receiver untouched, so
// values can be shared freely between goroutines. Updates cost O(log32 n)
// time and allocate only the path they change; the rest is shared. That is
// what makes `set xs = list.push(xs, x)` in a loop linear rather than
// quadratic, without giving up value semantics.
//
// Vec additionally supports exclusive ownership (Edit, PushMut, SetMut,
// Freeze): when the interpreter can prove that a vector is held by exactly one
// variable, it updates the vector in place, like a Clojure transient, and
// freezes it again the moment it may be observed by anything else.
package pds

const (
	shiftBits = 5
	width     = 1 << shiftBits
	mask      = width - 1
)

// Token identifies one exclusive owner. Nodes stamped with a token may be
// mutated by that owner; all other nodes are shared and must be copied.
type Token struct{ _ byte }

type node struct {
	kids []*node // internal node: up to 32 children
	vals []any   // leaf node: up to 32 values
	edit *Token
}

// Vec is an immutable vector: a 32-way trie plus a tail array that makes
// appends cheap.
type Vec struct {
	cnt   int
	shift uint
	root  *node
	tail  []any

	// A view is the window [off, off+cnt) of base, shared without copying.
	// Only Slice creates views; every operation that changes the contents
	// first flattens the view into an ordinary vector.
	base *Vec
	off  int

	// edit is non-nil while this header is exclusively owned (see Edit).
	edit *Token
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
func (v *Vec) Get(i int) any {
	if v.base != nil {
		return v.base.Get(v.off + i)
	}
	return v.arrayFor(i)[i&mask]
}

// flat returns v as an ordinary (non-view) vector.
func (v *Vec) flat() *Vec {
	if v.base == nil {
		return v
	}
	return FromSlice(v.ToSlice())
}

// Set returns a vector with element i replaced. 0 <= i < Len().
func (v *Vec) Set(i int, x any) *Vec {
	v = v.flat()
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
	v = v.flat()
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
		root = &node{kids: []*node{v.root, newPath(v.shift, tailNode, nil)}}
		shift += shiftBits
	} else {
		root = pushTail(v.cnt, v.shift, v.root, tailNode, nil)
	}
	tail := make([]any, 1, width)
	tail[0] = x
	return &Vec{cnt: v.cnt + 1, shift: shift, root: root, tail: tail}
}

func newPath(level uint, n *node, edit *Token) *node {
	if level == 0 {
		return n
	}
	return &node{kids: []*node{newPath(level-shiftBits, n, edit)}, edit: edit}
}

// pushTail inserts tailNode below parent. With a non-nil edit token, nodes
// stamped with that token are updated in place.
func pushTail(cnt int, level uint, parent, tailNode *node, edit *Token) *node {
	sub := ((cnt - 1) >> level) & mask
	var ret *node
	if edit != nil && parent.edit == edit {
		ret = parent
	} else {
		kids := make([]*node, len(parent.kids), max(len(parent.kids)+1, sub+1))
		copy(kids, parent.kids)
		ret = &node{kids: kids, edit: edit}
	}
	var ins *node
	switch {
	case level == shiftBits:
		ins = tailNode
	case sub < len(ret.kids):
		ins = pushTail(cnt, level-shiftBits, ret.kids[sub], tailNode, edit)
	default:
		ins = newPath(level-shiftBits, tailNode, edit)
	}
	if sub < len(ret.kids) {
		ret.kids[sub] = ins
	} else {
		ret.kids = append(ret.kids, ins)
	}
	return ret
}

// ---- exclusive ownership ----

// Owned reports whether v is exclusively owned, so PushMut and SetMut may
// change it in place.
func (v *Vec) Owned() bool { return v.edit != nil }

// Freeze ends exclusive ownership. Call it before v can be seen by anything
// other than its single owner; afterwards v is an ordinary immutable vector.
func (v *Vec) Freeze() { v.edit = nil }

// Edit returns a new exclusively owned vector with the same elements as v.
// It shares v's trie until the first mutation touches a node.
func (v *Vec) Edit() *Vec {
	v = v.flat()
	tail := make([]any, len(v.tail), width)
	copy(tail, v.tail)
	return &Vec{cnt: v.cnt, shift: v.shift, root: v.root, tail: tail, edit: &Token{}}
}

// PushMut appends x in place. v must be owned.
func (v *Vec) PushMut(x any) {
	if len(v.tail) < width {
		v.tail = append(v.tail, x)
		v.cnt++
		return
	}
	tailNode := &node{vals: v.tail, edit: v.edit}
	if (v.cnt >> shiftBits) > (1 << v.shift) {
		v.root = &node{kids: []*node{v.root, newPath(v.shift, tailNode, v.edit)}, edit: v.edit}
		v.shift += shiftBits
	} else {
		v.root = pushTail(v.cnt, v.shift, v.root, tailNode, v.edit)
	}
	v.tail = make([]any, 1, width)
	v.tail[0] = x
	v.cnt++
}

// SetMut replaces element i in place. v must be owned; 0 <= i < Len().
func (v *Vec) SetMut(i int, x any) {
	if i >= v.tailoff() {
		v.tail[i&mask] = x
		return
	}
	v.root = assocMut(v.edit, v.shift, v.root, i, x)
}

func assocMut(edit *Token, level uint, n *node, i int, x any) *node {
	if n.edit != edit {
		c := &node{edit: edit}
		if level == 0 {
			c.vals = append(make([]any, 0, width), n.vals...)
		} else {
			c.kids = append(make([]*node, 0, width), n.kids...)
		}
		n = c
	}
	if level == 0 {
		n.vals[i&mask] = x
		return n
	}
	sub := (i >> level) & mask
	n.kids[sub] = assocMut(edit, level-shiftBits, n.kids[sub], i, x)
	return n
}

// ---- iteration and slicing ----

// Range calls f for each element in order until f returns false.
func (v *Vec) Range(f func(i int, x any) bool) {
	base, off := v, 0
	if v.base != nil {
		base, off = v.base, v.off
	}
	for i := 0; i < v.cnt; {
		arr := base.arrayFor(off + i)
		start := (off + i) & mask
		n := min(len(arr)-start, v.cnt-i)
		for k := 0; k < n; k++ {
			if !f(i+k, arr[start+k]) {
				return
			}
		}
		i += n
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

// Slice returns elements [start, stop). Bounds must satisfy
// 0 <= start <= stop <= Len(). Large windows are shared, not copied.
func (v *Vec) Slice(start, stop int) *Vec {
	if start == 0 && stop == v.cnt {
		return v
	}
	n := stop - start
	if n <= 0 {
		return emptyVec
	}
	base, off := v, 0
	if v.base != nil {
		base, off = v.base, v.off
	}
	// Small or sparse windows are copied so that a view never keeps a much
	// larger vector alive.
	if n <= width || n*2 < base.cnt || v.edit != nil {
		out := make([]any, 0, n)
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
	return &Vec{cnt: n, base: base, off: off + start}
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
	out := v.flat()
	w.Range(func(_ int, x any) bool {
		out = out.Push(x)
		return true
	})
	return out
}
