package pds

import "math/bits"

// Entry is one key/value pair of a Map.
type Entry struct {
	Key, Val any
}

// Config supplies hashing and equality for a Map's keys. It is shared by all
// versions of a map.
type Config struct {
	Hash  func(any) uint64
	Equal func(a, b any) bool
}

// Map is an immutable, insertion-ordered hash map.
//
// Entries live in a persistent vector in insertion order (removed entries
// leave a nil tombstone that is compacted away once they dominate). A hash
// array mapped trie maps a key's hash to its position in that vector, so
// lookups, inserts, updates and removals are all O(log n).
type Map struct {
	cfg     *Config
	entries *Vec
	root    *hnode
	live    int
	edit    *Token // non-nil while exclusively owned (see Edit)
}

type hnode struct {
	bitmap uint32
	slots  []hslot
	edit   *Token
}

// hslot is a child node, or a bucket of entry positions sharing one hash.
type hslot struct {
	child *hnode
	hash  uint64
	idx   int
	more  *[]int // further positions with the same full hash (collisions), usually nil
}

// NewMap returns an empty map using cfg.
func NewMap(cfg *Config) *Map { return &Map{cfg: cfg, entries: emptyVec} }

// Len is the number of entries.
func (m *Map) Len() int { return m.live }

func (m *Map) find(k any, h uint64) (int, bool) {
	n := m.root
	for shift := uint(0); n != nil; shift += shiftBits {
		bit := uint32(1) << ((h >> shift) & mask)
		if n.bitmap&bit == 0 {
			return 0, false
		}
		s := &n.slots[bits.OnesCount32(n.bitmap&(bit-1))]
		if s.child != nil {
			n = s.child
			continue
		}
		if s.hash != h {
			return 0, false
		}
		if e := m.entries.Get(s.idx).(*Entry); m.cfg.Equal(e.Key, k) {
			return s.idx, true
		}
		for _, i := range s.extra() {
			if e := m.entries.Get(i).(*Entry); m.cfg.Equal(e.Key, k) {
				return i, true
			}
		}
		return 0, false
	}
	return 0, false
}

// Get returns the value stored under k.
func (m *Map) Get(k any) (any, bool) {
	if m.live == 0 {
		return nil, false
	}
	i, ok := m.find(k, m.cfg.Hash(k))
	if !ok {
		return nil, false
	}
	return m.entries.Get(i).(*Entry).Val, true
}

// Has reports whether k is present.
func (m *Map) Has(k any) bool {
	_, ok := m.Get(k)
	return ok
}

// Put returns a map with k set to v. An existing key keeps its position.
func (m *Map) Put(k, v any) *Map {
	h := m.cfg.Hash(k)
	if m.live > 0 {
		if i, ok := m.find(k, h); ok {
			old := m.entries.Get(i).(*Entry)
			return &Map{cfg: m.cfg, entries: m.entries.Set(i, &Entry{Key: old.Key, Val: v}), root: m.root, live: m.live}
		}
	}
	idx := m.entries.Len()
	return &Map{
		cfg:     m.cfg,
		entries: m.entries.Push(&Entry{Key: k, Val: v}),
		root:    insert(m.root, 0, h, idx),
		live:    m.live + 1,
	}
}

func insert(n *hnode, shift uint, h uint64, idx int) *hnode {
	bit := uint32(1) << ((h >> shift) & mask)
	if n == nil {
		return &hnode{bitmap: bit, slots: []hslot{{hash: h, idx: idx}}}
	}
	pos := bits.OnesCount32(n.bitmap & (bit - 1))
	if n.bitmap&bit == 0 {
		slots := make([]hslot, len(n.slots)+1)
		copy(slots, n.slots[:pos])
		slots[pos] = hslot{hash: h, idx: idx}
		copy(slots[pos+1:], n.slots[pos:])
		return &hnode{bitmap: n.bitmap | bit, slots: slots}
	}
	slots := append([]hslot(nil), n.slots...)
	s := slots[pos]
	switch {
	case s.child != nil:
		slots[pos] = hslot{child: insert(s.child, shift+shiftBits, h, idx)}
	case s.hash == h:
		more := append(append([]int(nil), s.extra()...), idx)
		s.more = &more
		slots[pos] = s
	default:
		// Two different hashes share this prefix: push both one level down.
		child := insert(nil, shift+shiftBits, s.hash, s.idx)
		for _, i := range s.extra() {
			child = addToBucket(child, shift+shiftBits, s.hash, i)
		}
		slots[pos] = hslot{child: insert(child, shift+shiftBits, h, idx)}
	}
	return &hnode{bitmap: n.bitmap, slots: slots}
}

func addToBucket(n *hnode, shift uint, h uint64, idx int) *hnode { return insert(n, shift, h, idx) }

// remove deletes position idx (whose key hashes to h) from the trie.
func remove(n *hnode, shift uint, h uint64, idx int) *hnode {
	bit := uint32(1) << ((h >> shift) & mask)
	if n == nil || n.bitmap&bit == 0 {
		return n
	}
	pos := bits.OnesCount32(n.bitmap & (bit - 1))
	s := n.slots[pos]
	var repl hslot
	keep := true
	switch {
	case s.child != nil:
		c := remove(s.child, shift+shiftBits, h, idx)
		if c == nil {
			keep = false
		} else {
			repl = hslot{child: c}
		}
	case s.idx == idx:
		if len(s.extra()) == 0 {
			keep = false
		} else {
			rest := s.extra()[1:]
			repl = hslot{hash: s.hash, idx: s.extra()[0]}
			if len(rest) > 0 {
				repl.more = &rest
			}
		}
	default:
		more := make([]int, 0, len(s.extra()))
		for _, i := range s.extra() {
			if i != idx {
				more = append(more, i)
			}
		}
		if len(more) == len(s.extra()) {
			return n
		}
		repl = hslot{hash: s.hash, idx: s.idx}
		if len(more) > 0 {
			repl.more = &more
		}
	}
	if keep {
		slots := append([]hslot(nil), n.slots...)
		slots[pos] = repl
		return &hnode{bitmap: n.bitmap, slots: slots}
	}
	if len(n.slots) == 1 {
		return nil
	}
	slots := make([]hslot, 0, len(n.slots)-1)
	slots = append(slots, n.slots[:pos]...)
	slots = append(slots, n.slots[pos+1:]...)
	return &hnode{bitmap: n.bitmap &^ bit, slots: slots}
}

// Remove returns a map without k.
func (m *Map) Remove(k any) *Map {
	if m.live == 0 {
		return m
	}
	h := m.cfg.Hash(k)
	i, ok := m.find(k, h)
	if !ok {
		return m
	}
	out := &Map{cfg: m.cfg, entries: m.entries.Set(i, nil), root: remove(m.root, 0, h, i), live: m.live - 1}
	if dead := out.entries.Len() - out.live; dead > 32 && dead > out.live {
		return out.compact()
	}
	return out
}

// compact rebuilds the map without tombstones.
func (m *Map) compact() *Map {
	out := NewMap(m.cfg)
	m.Range(func(k, v any) bool {
		out = out.Put(k, v)
		return true
	})
	return out
}

// Range calls f for each entry in insertion order until f returns false.
func (m *Map) Range(f func(k, v any) bool) {
	m.entries.Range(func(_ int, x any) bool {
		if e, _ := x.(*Entry); e != nil {
			return f(e.Key, e.Val)
		}
		return true
	})
}

// Keys returns the keys in insertion order.
func (m *Map) Keys() []any {
	out := make([]any, 0, m.live)
	m.Range(func(k, _ any) bool { out = append(out, k); return true })
	return out
}

// Values returns the values in insertion order.
func (m *Map) Values() []any {
	out := make([]any, 0, m.live)
	m.Range(func(_, v any) bool { out = append(out, v); return true })
	return out
}

func (s *hslot) extra() []int {
	if s.more == nil {
		return nil
	}
	return *s.more
}
