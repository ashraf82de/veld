package interp

import (
	"math"
	"strings"

	"github.com/ashraf82de/veld/internal/diag"
	"github.com/ashraf82de/veld/internal/syntax"
	"github.com/ashraf82de/veld/internal/types"
)

const (
	smallIntMin = -256
	smallIntMax = 16384
)

var smallInts = func() []Value {
	out := make([]Value, smallIntMax-smallIntMin)
	for i := range out {
		out[i] = int64(i + smallIntMin)
	}
	return out
}()

// mkInt boxes an Int, reusing preallocated boxes for small values so that
// counters and indices do not allocate.
func mkInt(i int64) Value {
	if i >= smallIntMin && i < smallIntMax {
		return smallInts[i-smallIntMin]
	}
	return i
}

func (c *compiler) typeOf(e syntax.Expr) types.Type {
	if t, ok := c.in.Prog.ExprTypes[e]; ok {
		return types.Prune(t)
	}
	return nil
}

func (c *compiler) is(e syntax.Expr, name string) bool {
	tc, ok := c.typeOf(e).(*types.TCon)
	return ok && tc.Info == nil && tc.Name == name
}

func readSlot(s int) code { return func(f *frame) Value { return f.slots[s] } }

// expr compiles e. Children that can exit the function (`return`, `?`, ...)
// are evaluated first into temporaries, so composite expressions never see a
// half-evaluated operand.
func (c *compiler) expr(e syntax.Expr) code {
	if o, ok := c.override[e]; ok {
		return o
	}
	if kids := c.hoistKids(e); len(kids) > 0 {
		return c.hoist(e, kids)
	}
	return c.expr0(e)
}

// hoistKids lists the unconditionally evaluated children of e, in order, if
// any of them may exit; otherwise nil.
func (c *compiler) hoistKids(e syntax.Expr) []syntax.Expr {
	var kids []syntax.Expr
	switch e := e.(type) {
	case *syntax.CallExpr:
		if !c.staticCallee(e.Fn) {
			kids = append(kids, e.Fn)
		}
		for _, a := range e.Args {
			kids = append(kids, a.Value)
		}
	case *syntax.PipeExpr:
		kids = append(kids, e.L)
		if !c.staticCallee(e.R.Fn) {
			kids = append(kids, e.R.Fn)
		}
		for _, a := range e.R.Args {
			kids = append(kids, a.Value)
		}
	case *syntax.BinaryExpr:
		if e.Op == syntax.KW_AND || e.Op == syntax.KW_OR {
			return nil
		}
		kids = []syntax.Expr{e.L, e.R}
	case *syntax.UnaryExpr:
		kids = []syntax.Expr{e.X}
	case *syntax.FieldExpr:
		if _, ok := c.moduleOf(e.X); ok {
			return nil
		}
		kids = []syntax.Expr{e.X}
	case *syntax.ListLit:
		kids = e.Elems
	case *syntax.MapLit:
		for _, en := range e.Entries {
			kids = append(kids, en.Key, en.Value)
		}
	case *syntax.RecordLit:
		if e.Base != nil {
			kids = append(kids, e.Base)
		}
		for _, f := range e.Fields {
			kids = append(kids, f.Value)
		}
	case *syntax.StrLit:
		for _, s := range e.Segs {
			if s.Expr != nil {
				kids = append(kids, s.Expr)
			}
		}
	default:
		return nil
	}
	for _, k := range kids {
		if c.mayExit(k) {
			return kids
		}
	}
	return nil
}

func (c *compiler) hoist(e syntax.Expr, kids []syntax.Expr) code {
	type step struct {
		slot  int
		run   code
		exits bool
	}
	steps := make([]step, len(kids))
	for i, k := range kids {
		steps[i] = step{slot: c.fs.newSlot(), run: c.expr(k), exits: c.mayExit(k)}
	}
	for i, k := range kids {
		c.override[k] = readSlot(steps[i].slot)
	}
	inner := c.expr0(e)
	for _, k := range kids {
		delete(c.override, k)
	}
	return func(f *frame) Value {
		for i := range steps {
			s := &steps[i]
			f.slots[s.slot] = s.run(f)
			if s.exits && f.ctl != ctlNone {
				return nil
			}
		}
		return inner(f)
	}
}

func (c *compiler) expr0(e syntax.Expr) code {
	switch e := e.(type) {
	case *syntax.IntLit:
		v := mkInt(e.Value)
		return func(*frame) Value { return v }
	case *syntax.FloatLit:
		var v Value = e.Value
		return func(*frame) Value { return v }
	case *syntax.BoolLit:
		var v Value = e.Value
		return func(*frame) Value { return v }
	case *syntax.StrLit:
		return c.strLit(e)
	case *syntax.ParenExpr:
		return c.expr(e.X)
	case *syntax.Ident:
		return c.ident(e)
	case *syntax.FieldExpr:
		return c.field(e)
	case *syntax.CallExpr:
		return c.call(e, nil, c.tailOK && c.tailNodes[e])
	case *syntax.PipeExpr:
		return c.call(e.R, e.L, c.tailOK && c.tailNodes[e])
	case *syntax.BinaryExpr:
		return c.binary(e)
	case *syntax.UnaryExpr:
		return c.unary(e)
	case *syntax.ListLit:
		elems := c.exprs(e.Elems)
		return func(f *frame) Value {
			out := make([]Value, len(elems))
			for i, x := range elems {
				out[i] = x(f)
			}
			return NewList(out)
		}
	case *syntax.MapLit:
		type entry struct{ k, v code }
		ents := make([]entry, len(e.Entries))
		for i, en := range e.Entries {
			ents[i] = entry{c.expr(en.Key), c.expr(en.Value)}
		}
		return func(f *frame) Value {
			m := NewMap()
			for _, en := range ents {
				m = m.Put(en.k(f), en.v(f))
			}
			return m
		}
	case *syntax.RecordLit:
		return c.record(e)
	case *syntax.LambdaExpr:
		return c.lambda(e)
	case *syntax.IfExpr:
		return c.ifExpr(e)
	case *syntax.MatchExpr:
		return c.match(e)
	case *syntax.ForExpr:
		return c.forExpr(e)
	case *syntax.WhileExpr:
		return c.whileExpr(e)
	case *syntax.ReturnExpr:
		return c.returnExpr(e)
	case *syntax.BreakExpr:
		return func(f *frame) Value { f.ctl = ctlBreak; return nil }
	case *syntax.ContinueExpr:
		return func(f *frame) Value { f.ctl = ctlContinue; return nil }
	case *syntax.TodoExpr:
		sp := e.Span
		return func(f *frame) Value {
			f.th.fail("R301", sp, "reached a `???` hole")
			return nil
		}
	case *syntax.TryExpr:
		return c.try(e)
	}
	panic("interp: cannot compile expression")
}

func (c *compiler) exprs(xs []syntax.Expr) []code {
	out := make([]code, len(xs))
	for i, x := range xs {
		out[i] = c.expr(x)
	}
	return out
}

func (c *compiler) strLit(e *syntax.StrLit) code {
	if len(e.Segs) == 1 && e.Segs[0].Expr == nil {
		var v Value = e.Segs[0].Lit
		return func(*frame) Value { return v }
	}
	if len(e.Segs) == 0 {
		var v Value = ""
		return func(*frame) Value { return v }
	}
	type part struct {
		lit string
		x   code
	}
	parts := make([]part, len(e.Segs))
	for i, s := range e.Segs {
		if s.Expr != nil {
			parts[i].x = c.expr(s.Expr)
		} else {
			parts[i].lit = s.Lit
		}
	}
	return func(f *frame) Value {
		var b strings.Builder
		for i := range parts {
			if p := &parts[i]; p.x != nil {
				b.WriteString(Show(p.x(f)))
			} else {
				b.WriteString(p.lit)
			}
		}
		return b.String()
	}
}

func (c *compiler) ident(e *syntax.Ident) code {
	if v := c.lookup(e.Name); v != nil {
		s := v.slot
		if v.boxed {
			return func(f *frame) Value { return f.slots[s].(*Box).v }
		}
		if v.strOwned {
			return func(f *frame) Value {
				if b, ok := f.slots[s].(*strBuf); ok {
					return b.str()
				}
				return f.slots[s]
			}
		}
		if v.owned && !c.noFreeze[e] {
			// Any read that is not known to keep the list to itself may let
			// it escape, so the variable gives up exclusive ownership.
			return func(f *frame) Value {
				x := f.slots[s]
				switch v := x.(type) {
				case List:
					v.Freeze()
				case Map:
					v.Freeze()
				}
				return x
			}
		}
		return func(f *frame) Value { return f.slots[s] }
	}
	if e.Name == "Unit" {
		return func(*frame) Value { return Unit }
	}
	if syntax.IsUpper(e.Name) {
		ci := c.ctor("", e.Name)
		if ci == nil {
			panic("interp: unknown constructor " + e.Name)
		}
		var v Value = &Variant{Ctor: ci}
		return func(*frame) Value { return v }
	}
	if fi := c.funcNamed(e.Name); fi != nil {
		var v Value = &FuncRef{Info: fi, fn: c.in.fns[fi]}
		return func(*frame) Value { return v }
	}
	panic("interp: unknown name " + e.Name)
}

// ctor finds a constructor by (optional) module qualifier.
func (c *compiler) ctor(module, name string) *types.CtorInfo {
	if module != "" {
		if m := c.mod.Imports[module]; m != nil {
			return m.Ctors[name]
		}
		return nil
	}
	if ci := c.mod.Ctors[name]; ci != nil {
		return ci
	}
	return c.in.Prog.Prelude.Ctors[name]
}

func (c *compiler) funcNamed(name string) *types.FuncInfo {
	if fi := c.mod.Funcs[name]; fi != nil {
		return fi
	}
	return c.in.Prog.Prelude.Funcs[name]
}

// moduleOf reports whether x names an imported module (and not a local).
func (c *compiler) moduleOf(x syntax.Expr) (*types.Module, bool) {
	id, ok := x.(*syntax.Ident)
	if !ok || c.fs.has(id.Name) {
		return nil, false
	}
	m, ok := c.mod.Imports[id.Name]
	return m, ok
}

func (c *compiler) field(e *syntax.FieldExpr) code {
	if m, ok := c.moduleOf(e.X); ok {
		if syntax.IsUpper(e.Name) {
			var v Value = &Variant{Ctor: m.Ctors[e.Name]}
			return func(*frame) Value { return v }
		}
		fi := m.Funcs[e.Name]
		if fi == nil {
			panic("interp: unknown member " + e.Name)
		}
		var v Value = &FuncRef{Info: fi, fn: c.in.fns[fi]}
		return func(*frame) Value { return v }
	}
	x := c.expr(e.X)
	name := e.Name
	if tc, ok := c.typeOf(e.X).(*types.TCon); ok && tc.Info != nil {
		if _, i, found := tc.Info.Field(name); found {
			return func(f *frame) Value { return x(f).(*Record).Fields[i] }
		}
	}
	return func(f *frame) Value {
		r := x(f).(*Record)
		_, i, _ := r.Type.Field(name)
		return r.Fields[i]
	}
}

func (c *compiler) record(e *syntax.RecordLit) code {
	var ti *types.TypeInfo
	if e.Module != "" {
		ti = c.mod.Imports[e.Module].Types[e.Name]
	} else if ti = c.mod.Types[e.Name]; ti == nil {
		ti = c.in.Prog.Prelude.Types[e.Name]
	}
	var base code
	if e.Base != nil {
		base = c.expr(e.Base)
	}
	idx := make([]int, len(e.Fields))
	vals := make([]code, len(e.Fields))
	for i, fi := range e.Fields {
		_, idx[i], _ = ti.Field(fi.Name)
		vals[i] = c.expr(fi.Value)
	}
	n := len(ti.Fields)
	return func(f *frame) Value {
		r := &Record{Type: ti, Fields: make([]Value, n)}
		if base != nil {
			copy(r.Fields, base(f).(*Record).Fields)
		}
		for i, v := range vals {
			r.Fields[idx[i]] = v(f)
		}
		return r
	}
}

// ---- operators ----

func (c *compiler) unary(e *syntax.UnaryExpr) code {
	if e.Op == syntax.KW_NOT {
		b := c.boolExpr(e)
		return func(f *frame) Value { return b(f) }
	}
	switch {
	case c.is(e, "Int"):
		i := c.intExpr(e)
		return func(f *frame) Value { return mkInt(i(f)) }
	case c.is(e, "Float"):
		x := c.floatExpr(e)
		return func(f *frame) Value { return x(f) }
	}
	x := c.expr(e.X)
	sp := e.Span
	return func(f *frame) Value {
		switch v := x(f).(type) {
		case int64:
			if v == math.MinInt64 {
				f.th.fail("R101", sp, "integer overflow in negation")
			}
			return mkInt(-v)
		case float64:
			return -v
		}
		return nil
	}
}

func (c *compiler) binary(e *syntax.BinaryExpr) code {
	switch e.Op {
	case syntax.KW_AND, syntax.KW_OR, syntax.EQ, syntax.NE, syntax.LT, syntax.LE, syntax.GT, syntax.GE:
		b := c.boolExpr(e)
		return func(f *frame) Value { return b(f) }
	}
	switch {
	case c.is(e.L, "Int"):
		i := c.intExpr(e)
		return func(f *frame) Value { return mkInt(i(f)) }
	case c.is(e.L, "Float"):
		x := c.floatExpr(e)
		return func(f *frame) Value { return x(f) }
	case c.is(e.L, "Str"):
		l, r := c.expr(e.L), c.expr(e.R)
		return func(f *frame) Value { return l(f).(string) + r(f).(string) }
	}
	l, r := c.expr(e.L), c.expr(e.R)
	op, opSp, sp := e.Op, e.OpSpan, e.Span
	return func(f *frame) Value {
		a, b := l(f), r(f)
		switch x := a.(type) {
		case string:
			return x + b.(string)
		case List:
			return x.Concat(b.(List))
		case int64:
			return mkInt(intArith(f.th, op, opSp, x, b.(int64)))
		case float64:
			return floatArith(op, x, b.(float64))
		}
		f.th.fail("R999", sp, "bad operands for %s", op)
		return nil
	}
}

func intArith(th *Thread, op syntax.Kind, sp diag.Span, a, b int64) int64 {
	switch op {
	case syntax.PLUS:
		s := a + b
		if (a^s)&(b^s) < 0 {
			th.fail("R101", sp, "integer overflow: %d + %d", a, b)
		}
		return s
	case syntax.MINUS:
		s := a - b
		if (a^b)&(a^s) < 0 {
			th.fail("R101", sp, "integer overflow: %d - %d", a, b)
		}
		return s
	case syntax.STAR:
		return mulInt(th, sp, a, b)
	case syntax.SLASH, syntax.PERCENT:
		if b == 0 {
			th.fail("R102", sp, "division by zero")
		}
		if a == math.MinInt64 && b == -1 {
			th.fail("R101", sp, "integer overflow in division")
		}
		if op == syntax.SLASH {
			return a / b
		}
		return a % b
	}
	return 0
}

func mulInt(th *Thread, sp diag.Span, a, b int64) int64 {
	if a == 0 || b == 0 {
		return 0
	}
	p := a * b
	if p/b != a || (a == -1 && b == math.MinInt64) || (b == -1 && a == math.MinInt64) {
		th.fail("R101", sp, "integer overflow: %d * %d", a, b)
	}
	return p
}

func floatArith(op syntax.Kind, a, b float64) Value {
	switch op {
	case syntax.PLUS:
		return a + b
	case syntax.MINUS:
		return a - b
	case syntax.STAR:
		return a * b
	case syntax.SLASH:
		return a / b
	case syntax.PERCENT:
		return math.Mod(a, b)
	}
	return nil
}

// ---- typed compilation ----
//
// Expressions whose static type is Int, Float or Bool are compiled to Go
// functions returning the unboxed value, so arithmetic and conditions run
// without allocating.

func (c *compiler) boolExpr(e syntax.Expr) bcode {
	if o, ok := c.override[e]; ok {
		return func(f *frame) bool { return o(f).(bool) }
	}
	switch e := e.(type) {
	case *syntax.BoolLit:
		k := e.Value
		return func(*frame) bool { return k }
	case *syntax.ParenExpr:
		return c.boolExpr(e.X)
	case *syntax.Ident:
		if v := c.lookup(e.Name); v != nil {
			s := v.slot
			if v.boxed {
				return func(f *frame) bool { return f.slots[s].(*Box).v.(bool) }
			}
			return func(f *frame) bool { return f.slots[s].(bool) }
		}
	case *syntax.UnaryExpr:
		if e.Op == syntax.KW_NOT {
			x := c.boolExpr(e.X)
			return func(f *frame) bool { return !x(f) }
		}
	case *syntax.BinaryExpr:
		switch e.Op {
		case syntax.KW_AND:
			l, r := c.boolExpr(e.L), c.boolExpr(e.R)
			if c.mayExit(e.L) || c.mayExit(e.R) {
				return func(f *frame) bool { return l(f) && f.ctl == ctlNone && r(f) }
			}
			return func(f *frame) bool { return l(f) && r(f) }
		case syntax.KW_OR:
			l, r := c.boolExpr(e.L), c.boolExpr(e.R)
			if c.mayExit(e.L) || c.mayExit(e.R) {
				return func(f *frame) bool { return (l(f) && f.ctl == ctlNone) || (f.ctl == ctlNone && r(f)) }
			}
			return func(f *frame) bool { return l(f) || r(f) }
		case syntax.EQ, syntax.NE, syntax.LT, syntax.LE, syntax.GT, syntax.GE:
			return c.compare(e)
		}
	}
	g := c.expr(e)
	if c.mayExit(e) {
		return func(f *frame) bool { b, _ := g(f).(bool); return b }
	}
	return func(f *frame) bool { return g(f).(bool) }
}

func (c *compiler) compare(e *syntax.BinaryExpr) bcode {
	op := e.Op
	switch {
	case c.is(e.L, "Int") && !c.mayExit(e):
		return c.compareInt(e)
	case c.is(e.L, "Float") && !c.mayExit(e):
		l, r := c.floatExpr(e.L), c.floatExpr(e.R)
		switch op {
		case syntax.EQ:
			return func(f *frame) bool { return l(f) == r(f) }
		case syntax.NE:
			return func(f *frame) bool { return l(f) != r(f) }
		case syntax.LT:
			return func(f *frame) bool { return l(f) < r(f) }
		case syntax.LE:
			return func(f *frame) bool { return l(f) <= r(f) }
		case syntax.GT:
			return func(f *frame) bool { return l(f) > r(f) }
		}
		return func(f *frame) bool { return l(f) >= r(f) }
	case c.is(e.L, "Str") && !c.mayExit(e):
		l, r := c.strExpr(e.L), c.strExpr(e.R)
		switch op {
		case syntax.EQ:
			return func(f *frame) bool { return l(f) == r(f) }
		case syntax.NE:
			return func(f *frame) bool { return l(f) != r(f) }
		case syntax.LT:
			return func(f *frame) bool { return l(f) < r(f) }
		case syntax.LE:
			return func(f *frame) bool { return l(f) <= r(f) }
		case syntax.GT:
			return func(f *frame) bool { return l(f) > r(f) }
		}
		return func(f *frame) bool { return l(f) >= r(f) }
	case c.is(e.L, "Bool") && (op == syntax.EQ || op == syntax.NE) && !c.mayExit(e):
		l, r := c.boolExpr(e.L), c.boolExpr(e.R)
		if op == syntax.EQ {
			return func(f *frame) bool { return l(f) == r(f) }
		}
		return func(f *frame) bool { return l(f) != r(f) }
	}
	l, r := c.expr(e.L), c.expr(e.R)
	switch op {
	case syntax.EQ:
		return func(f *frame) bool { return Equal(l(f), r(f)) }
	case syntax.NE:
		return func(f *frame) bool { return !Equal(l(f), r(f)) }
	case syntax.LT:
		return func(f *frame) bool { return Compare(l(f), r(f)) < 0 }
	case syntax.LE:
		return func(f *frame) bool { return Compare(l(f), r(f)) <= 0 }
	case syntax.GT:
		return func(f *frame) bool { return Compare(l(f), r(f)) > 0 }
	}
	return func(f *frame) bool { return Compare(l(f), r(f)) >= 0 }
}

func (c *compiler) compareInt(e *syntax.BinaryExpr) bcode {
	l, r := c.intExpr(e.L), c.intExpr(e.R)
	// `local op constant` is by far the most common shape in loops.
	if lid, ok := e.L.(*syntax.Ident); ok {
		if k, ok := e.R.(*syntax.IntLit); ok {
			if v := c.lookup(lid.Name); v != nil && !v.boxed {
				s, kv := v.slot, k.Value
				switch e.Op {
				case syntax.EQ:
					return func(f *frame) bool { return f.slots[s].(int64) == kv }
				case syntax.NE:
					return func(f *frame) bool { return f.slots[s].(int64) != kv }
				case syntax.LT:
					return func(f *frame) bool { return f.slots[s].(int64) < kv }
				case syntax.LE:
					return func(f *frame) bool { return f.slots[s].(int64) <= kv }
				case syntax.GT:
					return func(f *frame) bool { return f.slots[s].(int64) > kv }
				}
				return func(f *frame) bool { return f.slots[s].(int64) >= kv }
			}
		}
	}
	if k, ok := e.R.(*syntax.IntLit); ok {
		kv := k.Value
		switch e.Op {
		case syntax.EQ:
			return func(f *frame) bool { return l(f) == kv }
		case syntax.NE:
			return func(f *frame) bool { return l(f) != kv }
		case syntax.LT:
			return func(f *frame) bool { return l(f) < kv }
		case syntax.LE:
			return func(f *frame) bool { return l(f) <= kv }
		case syntax.GT:
			return func(f *frame) bool { return l(f) > kv }
		}
		return func(f *frame) bool { return l(f) >= kv }
	}
	switch e.Op {
	case syntax.EQ:
		return func(f *frame) bool { return l(f) == r(f) }
	case syntax.NE:
		return func(f *frame) bool { return l(f) != r(f) }
	case syntax.LT:
		return func(f *frame) bool { return l(f) < r(f) }
	case syntax.LE:
		return func(f *frame) bool { return l(f) <= r(f) }
	case syntax.GT:
		return func(f *frame) bool { return l(f) > r(f) }
	}
	return func(f *frame) bool { return l(f) >= r(f) }
}

func (c *compiler) strExpr(e syntax.Expr) func(f *frame) string {
	if o, ok := c.override[e]; ok {
		return func(f *frame) string { return o(f).(string) }
	}
	switch e := e.(type) {
	case *syntax.ParenExpr:
		return c.strExpr(e.X)
	case *syntax.StrLit:
		if len(e.Segs) == 1 && e.Segs[0].Expr == nil {
			k := e.Segs[0].Lit
			return func(*frame) string { return k }
		}
	}
	g := c.expr(e)
	return func(f *frame) string { return g(f).(string) }
}

func (c *compiler) intExpr(e syntax.Expr) icode {
	if o, ok := c.override[e]; ok {
		return func(f *frame) int64 { return o(f).(int64) }
	}
	switch e := e.(type) {
	case *syntax.IntLit:
		k := e.Value
		return func(*frame) int64 { return k }
	case *syntax.ParenExpr:
		return c.intExpr(e.X)
	case *syntax.Ident:
		if v := c.lookup(e.Name); v != nil {
			s := v.slot
			if v.boxed {
				return func(f *frame) int64 { return f.slots[s].(*Box).v.(int64) }
			}
			return func(f *frame) int64 { return f.slots[s].(int64) }
		}
	case *syntax.UnaryExpr:
		if e.Op == syntax.MINUS && c.is(e, "Int") {
			x, sp := c.intExpr(e.X), e.Span
			return func(f *frame) int64 {
				v := x(f)
				if v == math.MinInt64 {
					f.th.fail("R101", sp, "integer overflow in negation")
				}
				return -v
			}
		}
	case *syntax.BinaryExpr:
		if c.is(e, "Int") && !c.mayExit(e) {
			switch e.Op {
			case syntax.PLUS, syntax.MINUS, syntax.STAR, syntax.SLASH, syntax.PERCENT:
				return c.intArithExpr(e)
			}
		}
	}
	g := c.expr(e)
	if c.mayExit(e) {
		return func(f *frame) int64 { x, _ := g(f).(int64); return x }
	}
	return func(f *frame) int64 { return g(f).(int64) }
}

func (c *compiler) intArithExpr(e *syntax.BinaryExpr) icode {
	l, r := c.intExpr(e.L), c.intExpr(e.R)
	sp := e.OpSpan
	if k, ok := e.R.(*syntax.IntLit); ok {
		kv := k.Value
		switch e.Op {
		case syntax.PLUS:
			return func(f *frame) int64 {
				a := l(f)
				s := a + kv
				if (a^s)&(kv^s) < 0 {
					f.th.fail("R101", sp, "integer overflow: %d + %d", a, kv)
				}
				return s
			}
		case syntax.MINUS:
			return func(f *frame) int64 {
				a := l(f)
				s := a - kv
				if (a^kv)&(a^s) < 0 {
					f.th.fail("R101", sp, "integer overflow: %d - %d", a, kv)
				}
				return s
			}
		case syntax.SLASH, syntax.PERCENT:
			if kv != 0 && kv != -1 {
				if e.Op == syntax.SLASH {
					return func(f *frame) int64 { return l(f) / kv }
				}
				return func(f *frame) int64 { return l(f) % kv }
			}
		}
	}
	switch e.Op {
	case syntax.PLUS:
		return func(f *frame) int64 {
			a, b := l(f), r(f)
			s := a + b
			if (a^s)&(b^s) < 0 {
				f.th.fail("R101", sp, "integer overflow: %d + %d", a, b)
			}
			return s
		}
	case syntax.MINUS:
		return func(f *frame) int64 {
			a, b := l(f), r(f)
			s := a - b
			if (a^b)&(a^s) < 0 {
				f.th.fail("R101", sp, "integer overflow: %d - %d", a, b)
			}
			return s
		}
	case syntax.STAR:
		return func(f *frame) int64 { return mulInt(f.th, sp, l(f), r(f)) }
	}
	op := e.Op
	return func(f *frame) int64 { return intArith(f.th, op, sp, l(f), r(f)) }
}

func (c *compiler) floatExpr(e syntax.Expr) fcode {
	if o, ok := c.override[e]; ok {
		return func(f *frame) float64 { return o(f).(float64) }
	}
	switch e := e.(type) {
	case *syntax.FloatLit:
		k := e.Value
		return func(*frame) float64 { return k }
	case *syntax.ParenExpr:
		return c.floatExpr(e.X)
	case *syntax.Ident:
		if v := c.lookup(e.Name); v != nil {
			s := v.slot
			if v.boxed {
				return func(f *frame) float64 { return f.slots[s].(*Box).v.(float64) }
			}
			return func(f *frame) float64 { return f.slots[s].(float64) }
		}
	case *syntax.UnaryExpr:
		if e.Op == syntax.MINUS && c.is(e, "Float") {
			x := c.floatExpr(e.X)
			return func(f *frame) float64 { return -x(f) }
		}
	case *syntax.BinaryExpr:
		if c.is(e, "Float") && !c.mayExit(e) {
			l, r := c.floatExpr(e.L), c.floatExpr(e.R)
			switch e.Op {
			case syntax.PLUS:
				return func(f *frame) float64 { return l(f) + r(f) }
			case syntax.MINUS:
				return func(f *frame) float64 { return l(f) - r(f) }
			case syntax.STAR:
				return func(f *frame) float64 { return l(f) * r(f) }
			case syntax.SLASH:
				return func(f *frame) float64 { return l(f) / r(f) }
			case syntax.PERCENT:
				return func(f *frame) float64 { return math.Mod(l(f), r(f)) }
			}
		}
	}
	g := c.expr(e)
	if c.mayExit(e) {
		return func(f *frame) float64 { x, _ := g(f).(float64); return x }
	}
	return func(f *frame) float64 { return g(f).(float64) }
}

// recordType resolves a record type by (optional) module qualifier.
func (c *compiler) recordType(module, name string) *types.TypeInfo {
	if module != "" {
		return c.mod.Imports[module].Types[name]
	}
	if ti := c.mod.Types[name]; ti != nil {
		return ti
	}
	return c.in.Prog.Prelude.Types[name]
}
