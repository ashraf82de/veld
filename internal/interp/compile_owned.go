package interp

import (
	"github.com/ashraf82de/veld/internal/syntax"
	"github.com/ashraf82de/veld/internal/types"
)

// In-place list updates.
//
// Lists are immutable values, but `set xs = list.push(xs, x)` and
// `set xs = list.set_at(xs, ...)` on a local `var` are the idiom for building
// and updating arrays, and copying the path to the changed element on every
// iteration is much slower than mutating it. The compiler therefore marks such
// a variable as owned: its slot holds the only reference to a vector that the
// update functions may change in place. Reading the variable anywhere else
// (passing it on, returning it, interpolating it, matching on it, ...) freezes
// the vector first, turning it back into an ordinary immutable value, so no
// other part of the program can ever observe a mutation. The next update after
// a freeze starts from a cheap structure-sharing copy.

// keepsArg lists functions that inspect a list argument without retaining it
// or returning it (or a view of it), so passing an owned variable to them does
// not need to freeze it.
var keepsArg = map[string]bool{
	"list.len": true, "list.get": true, "list.first": true, "list.last": true,
	"list.is_empty": true, "list.contains": true, "list.index_of": true,
	"list.any": true, "list.all": true, "list.count": true, "list.find": true,
	"list.find_index": true, "list.map": true, "list.map_indexed": true,
	"list.filter": true, "list.fold": true, "list.reverse": true, "list.sort": true,
	"list.sort_by": true, "list.max": true, "list.min": true, "list.sum": true,
	"list.sum_float": true, "list.unique": true, "str.join": true,
}

// markNoFreeze records that the first argument of a call to a function in
// keepsArg may read an owned variable without freezing it.
func (c *compiler) markNoFreeze(fn syntax.Expr, args []*syntax.Arg) {
	fi := c.staticFunc(fn)
	if fi == nil || !keepsArg[fi.QualName()] || len(fi.Params) == 0 {
		return
	}
	pos, _ := placeArgs(args, paramNames(fi))
	for i, a := range args {
		if pos[i] == 0 {
			if id, ok := a.Value.(*syntax.Ident); ok {
				c.noFreeze[id] = true
			}
		}
	}
}

func paramNames(fi *types.FuncInfo) []string {
	names := make([]string, len(fi.Params))
	for i, p := range fi.Params {
		names[i] = p.Name
	}
	return names
}

// inplace compiles `set xs = list.push(xs, x)` and
// `set xs = list.set_at(xs, index: i, item: x)` when xs is a local var. The
// first compilation pass only records that xs is updated this way (the
// variable must be known to be owned before any of its reads are compiled);
// later passes emit the in-place code.
func (c *compiler) inplace(s *syntax.SetStmt) (code, bool) {
	var call *syntax.CallExpr
	var piped syntax.Expr
	switch v := s.Value.(type) {
	case *syntax.CallExpr:
		call = v
	case *syntax.PipeExpr:
		call, piped = v.R, v.L
	default:
		return nil, false
	}
	fi := c.staticFunc(call.Fn)
	if fi == nil {
		return nil, false
	}
	name := fi.QualName()
	if name != "list.push" && name != "list.set_at" {
		return nil, false
	}
	args := call.Args
	if piped != nil {
		args = append([]*syntax.Arg{{Value: piped}}, args...)
	}
	names := paramNames(fi)
	if len(args) != len(names) {
		return nil, false
	}
	pos, _ := placeArgs(args, names)
	byPos := make([]syntax.Expr, len(names))
	for i, a := range args {
		byPos[pos[i]] = a.Value
	}
	id, ok := byPos[0].(*syntax.Ident)
	if !ok || id.Name != s.Name {
		return nil, false
	}
	v := c.lookup(s.Name)
	if v == nil || v.decl == nil || !v.mutable || v.boxed {
		return nil, false
	}
	if !c.owned[v.decl] {
		c.owned[v.decl] = true
		c.grew = true
		return nil, false
	}
	for _, x := range byPos[1:] {
		if c.mayExit(x) {
			return nil, false
		}
	}
	slot := v.slot
	site := &callSite{name: name, span: call.Span}
	if name == "list.push" {
		item := c.expr(byPos[1])
		return func(f *frame) Value {
			x := item(f)
			l := f.slots[slot].(List)
			if !l.Owned() {
				l = l.Edit()
				f.slots[slot] = l
			}
			l.PushMut(x)
			return Unit
		}, true
	}
	index, item := c.intExpr(byPos[1]), c.expr(byPos[2])
	return func(f *frame) Value {
		i := index(f)
		x := item(f)
		l := f.slots[slot].(List)
		if i < 0 || i >= int64(l.Len()) {
			th := f.th
			th.push(name, site)
			th.fail("R300", site.span, "list.set_at: index %d out of range for list of length %d", i, l.Len())
		}
		if !l.Owned() {
			l = l.Edit()
			f.slots[slot] = l
		}
		l.SetMut(int(i), x)
		return Unit
	}, true
}
