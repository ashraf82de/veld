package pds

import "math/bits"

// Exclusive ownership for maps, mirroring Vec (see Vec.Edit): a map held by a
// single variable can be updated in place, and must be frozen the moment
// anything else may see it.

// Owned reports whether m is exclusively owned.
func (m *Map) Owned() bool { return m.edit != nil }

// Freeze ends exclusive ownership; afterwards m is an ordinary immutable map.
func (m *Map) Freeze() {
	m.edit = nil
	m.entries.Freeze()
}

// Edit returns a new exclusively owned map with the same entries as m. It
// shares m's storage until the first mutation touches a node.
func (m *Map) Edit() *Map {
	return &Map{cfg: m.cfg, entries: m.entries.Edit(), root: m.root, live: m.live, edit: &Token{}}
}

// PutMut sets k to v in place. m must be owned.
func (m *Map) PutMut(k, v any) {
	h := m.cfg.Hash(k)
	if m.live > 0 {
		if i, ok := m.find(k, h); ok {
			old := m.entries.Get(i).(*Entry)
			m.entries.SetMut(i, &Entry{Key: old.Key, Val: v})
			return
		}
	}
	idx := m.entries.Len()
	m.entries.PushMut(&Entry{Key: k, Val: v})
	m.root = insertMut(m.root, 0, h, idx, m.edit)
	m.live++
}

// RemoveMut deletes k in place and returns the (owned) map to keep using,
// which is a new one when the tombstones were compacted away. m must be owned.
func (m *Map) RemoveMut(k any) *Map {
	if m.live == 0 {
		return m
	}
	h := m.cfg.Hash(k)
	i, ok := m.find(k, h)
	if !ok {
		return m
	}
	m.entries.SetMut(i, nil)
	m.root = remove(m.root, 0, h, i)
	m.live--
	if dead := m.entries.Len() - m.live; dead > 32 && dead > m.live {
		return m.compact().Edit()
	}
	return m
}

func insertMut(n *hnode, shift uint, h uint64, idx int, tok *Token) *hnode {
	bit := uint32(1) << ((h >> shift) & mask)
	if n == nil {
		return &hnode{bitmap: bit, slots: []hslot{{hash: h, idx: idx}}, edit: tok}
	}
	if n.edit != tok {
		n = &hnode{bitmap: n.bitmap, slots: append(make([]hslot, 0, len(n.slots)+1), n.slots...), edit: tok}
	}
	pos := bits.OnesCount32(n.bitmap & (bit - 1))
	if n.bitmap&bit == 0 {
		n.slots = append(n.slots, hslot{})
		copy(n.slots[pos+1:], n.slots[pos:])
		n.slots[pos] = hslot{hash: h, idx: idx}
		n.bitmap |= bit
		return n
	}
	s := n.slots[pos]
	switch {
	case s.child != nil:
		n.slots[pos] = hslot{child: insertMut(s.child, shift+shiftBits, h, idx, tok)}
	case s.hash == h:
		more := append(append([]int(nil), s.extra()...), idx)
		s.more = &more
		n.slots[pos] = s
	default:
		child := insert(nil, shift+shiftBits, s.hash, s.idx)
		for _, i := range s.extra() {
			child = addToBucket(child, shift+shiftBits, s.hash, i)
		}
		n.slots[pos] = hslot{child: insert(child, shift+shiftBits, h, idx)}
	}
	return n
}
