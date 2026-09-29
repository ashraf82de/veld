package interp

import (
	"strings"

	"github.com/ashraf82de/veld/internal/syntax"
	"github.com/ashraf82de/veld/internal/types"
)

// ---- calls ----

func (c *compiler) staticCallee(fn syntax.Expr) bool {
	switch f := fn.(type) {
	case *syntax.Ident:
		return true
	case *syntax.FieldExpr:
		_, ok := c.moduleOf(f.X)
		return ok
	}
	return false
}

// ctorOfCallee returns the constructor a call targets, if any.
func (c *compiler) ctorOfCallee(fn syntax.Expr) *types.CtorInfo {
	switch f := fn.(type) {
	case *syntax.Ident:
		if syntax.IsUpper(f.Name) && !c.fs.has(f.Name) {
			return c.ctor("", f.Name)
		}
	case *syntax.FieldExpr:
		if syntax.IsUpper(f.Name) {
			if m, ok := c.moduleOf(f.X); ok {
				return m.Ctors[f.Name]
			}
		}
	}
	return nil
}

// staticFunc returns the top-level function a call targets, if it is known
// at compile time.
func (c *compiler) staticFunc(fn syntax.Expr) *types.FuncInfo {
	switch f := fn.(type) {
	case *syntax.Ident:
		if !syntax.IsUpper(f.Name) && !c.fs.has(f.Name) {
			return c.funcNamed(f.Name)
		}
	case *syntax.FieldExpr:
		if !syntax.IsUpper(f.Name) {
			if m, ok := c.moduleOf(f.X); ok {
				return m.Funcs[f.Name]
			}
		}
	}
	return nil
}

func fieldNames(fs []types.FieldInfo) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Name
	}
	return out
}

// placeArgs maps each written argument to its parameter position: positional
// arguments keep their index, named ones go where their name says.
func placeArgs(args []*syntax.Arg, names []string) (pos []int, n int) {
	n = max(len(names), len(args))
	pos = make([]int, len(args))
	for j, a := range args {
		pos[j] = j
		if a.Name == "" {
			continue
		}
		for k, nm := range names {
			if nm == a.Name {
				pos[j] = k
				break
			}
		}
	}
	return pos, n
}

func (c *compiler) call(e *syntax.CallExpr, piped syntax.Expr, tail bool) code {
	args := e.Args
	if piped != nil {
		args = append([]*syntax.Arg{{Value: piped}}, args...)
	}
	site := &callSite{name: "call", span: e.Span}
	c.markNoFreeze(e.Fn, args)
	argc := make([]code, len(args))
	for i, a := range args {
		argc[i] = c.expr(a.Value)
	}

	if ci := c.ctorOfCallee(e.Fn); ci != nil {
		pos, n := placeArgs(args, fieldNames(ci.Fields))
		return func(f *frame) Value {
			fields := make([]Value, n)
			for i, a := range argc {
				fields[pos[i]] = a(f)
			}
			return &Variant{Ctor: ci, Fields: fields}
		}
	}

	if fi := c.staticFunc(e.Fn); fi != nil {
		fn := c.in.fns[fi]
		site.name = fn.name
		names := make([]string, len(fi.Params))
		for i, p := range fi.Params {
			names[i] = p.Name
		}
		pos, n := placeArgs(args, names)
		if fi.Extern {
			return func(f *frame) Value {
				vals := make([]Value, n)
				for i, a := range argc {
					vals[pos[i]] = a(f)
				}
				return f.th.callNative(fn, vals, site)
			}
		}
		if tail {
			return func(f *frame) Value {
				th := f.th
				nf := th.newFrame(fn.nslots)
				for i, a := range argc {
					nf.slots[pos[i]] = a(f)
				}
				f.tailFrame, f.tailFn, f.tailSite = nf, fn, site
				f.ctl = ctlTail
				return nil
			}
		}
		switch len(argc) {
		case 1:
			a0, p0 := argc[0], pos[0]
			return func(f *frame) Value {
				th := f.th
				nf := th.newFrame(fn.nslots)
				nf.slots[p0] = a0(f)
				return th.invoke(fn, nf, site)
			}
		case 2:
			a0, a1, p0, p1 := argc[0], argc[1], pos[0], pos[1]
			return func(f *frame) Value {
				th := f.th
				nf := th.newFrame(fn.nslots)
				nf.slots[p0] = a0(f)
				nf.slots[p1] = a1(f)
				return th.invoke(fn, nf, site)
			}
		}
		return func(f *frame) Value {
			th := f.th
			nf := th.newFrame(fn.nslots)
			for i, a := range argc {
				nf.slots[pos[i]] = a(f)
			}
			return th.invoke(fn, nf, site)
		}
	}

	callee := c.expr(e.Fn)
	named := false
	for _, a := range args {
		named = named || a.Name != ""
	}
	return func(f *frame) Value {
		fv := callee(f)
		vals := make([]Value, len(argc))
		for i, a := range argc {
			vals[i] = a(f)
		}
		if named {
			if fr, ok := fv.(*FuncRef); ok {
				names := make([]string, len(fr.Info.Params))
				for i, p := range fr.Info.Params {
					names[i] = p.Name
				}
				pos, n := placeArgs(args, names)
				placed := make([]Value, n)
				for i, v := range vals {
					placed[pos[i]] = v
				}
				vals = placed
			}
		}
		return f.th.callValue(fv, vals, site)
	}
}

func (c *compiler) lambda(e *syntax.LambdaExpr) code {
	outer := c.fs
	fs := newFnScope(outer)
	c.fs = fs
	for _, p := range e.Params {
		c.declare(p.Name, false, nil)
	}
	body := c.block(e.Body)
	c.fs = outer
	l := &lambdaCode{nparams: len(e.Params), nslots: fs.nslots, capSlots: fs.capSlots, body: body}
	srcs := make([]int, len(fs.capFrom))
	for i, v := range fs.capFrom {
		srcs[i] = v.slot
	}
	if len(srcs) == 0 {
		var v Value = &Closure{fn: l}
		return func(*frame) Value { return v }
	}
	return func(f *frame) Value {
		caps := make([]Value, len(srcs))
		for i, s := range srcs {
			caps[i] = f.slots[s]
		}
		return &Closure{fn: l, caps: caps}
	}
}

// ---- control flow ----

func (c *compiler) ifExpr(e *syntax.IfExpr) code {
	n := len(e.Branches)
	conds := make([]bcode, n)
	bodies := make([]code, n)
	anyExit := false
	for i, br := range e.Branches {
		conds[i] = c.boolExpr(br.Cond)
		anyExit = anyExit || c.mayExit(br.Cond)
		bodies[i] = c.block(br.Body)
	}
	var els code
	if e.Else != nil {
		els = c.block(e.Else)
	}
	if n == 1 && !anyExit {
		cond, body := conds[0], bodies[0]
		if els != nil {
			return func(f *frame) Value {
				if cond(f) {
					return body(f)
				}
				return els(f)
			}
		}
		return func(f *frame) Value {
			if cond(f) {
				return body(f)
			}
			return Unit
		}
	}
	return func(f *frame) Value {
		for i, cond := range conds {
			ok := cond(f)
			if anyExit && f.ctl != ctlNone {
				return nil
			}
			if ok {
				return bodies[i](f)
			}
		}
		if els != nil {
			return els(f)
		}
		return Unit
	}
}

type matcher func(f *frame, v Value) bool

type armCode struct {
	pat   matcher
	guard bcode
	body  code
}

// matchMulti compiles `match a, b`: the values are matched column by column,
// without building the TupleN constructor the checker sees.
func (c *compiler) matchMulti(e *syntax.MatchExpr) code {
	n := len(e.Values)
	scruts := c.exprs(e.Values)
	exits := false
	for _, v := range e.Values {
		exits = exits || c.mayExit(v)
	}
	type multiArm struct {
		pats  []matcher
		guard bcode
		body  code
	}
	arms := make([]multiArm, len(e.Arms))
	for i, a := range e.Arms {
		c.fs.push()
		tp := a.Pattern.(*syntax.CtorPat)
		arms[i].pats = make([]matcher, n)
		for j, sub := range tp.Args {
			arms[i].pats[j] = c.pattern(sub)
		}
		if a.Guard != nil {
			arms[i].guard = c.boolExpr(a.Guard)
		}
		arms[i].body = c.block(a.Body)
		c.fs.pop()
	}
	sp := e.Span
	return func(f *frame) Value {
		var vals [4]Value
		for i, s := range scruts {
			vals[i] = s(f)
			if exits && f.ctl != ctlNone {
				return nil
			}
		}
	next:
		for i := range arms {
			a := &arms[i]
			for j, m := range a.pats {
				if m != nil && !m(f, vals[j]) {
					continue next
				}
			}
			if a.guard != nil && !a.guard(f) {
				continue
			}
			return a.body(f)
		}
		parts := make([]string, n)
		for i := range parts {
			parts[i] = Repr(vals[i])
		}
		f.th.fail("R303", sp, "no case matched values %s", strings.Join(parts, ", "))
		return nil
	}
}

func (c *compiler) match(e *syntax.MatchExpr) code {
	if e.Values != nil {
		return c.matchMulti(e)
	}
	scrut := c.expr(e.X)
	exits := c.mayExit(e.X)
	arms := make([]armCode, len(e.Arms))
	for i, a := range e.Arms {
		c.fs.push()
		arms[i].pat = c.pattern(a.Pattern)
		if a.Guard != nil {
			arms[i].guard = c.boolExpr(a.Guard)
		}
		arms[i].body = c.block(a.Body)
		c.fs.pop()
	}
	sp := e.Span
	return func(f *frame) Value {
		v := scrut(f)
		if exits && f.ctl != ctlNone {
			return nil
		}
		for i := range arms {
			a := &arms[i]
			if a.pat != nil && !a.pat(f, v) {
				continue
			}
			if a.guard != nil && !a.guard(f) {
				continue
			}
			return a.body(f)
		}
		f.th.fail("R303", sp, "no case matched value %s", Repr(v))
		return nil
	}
}

func (c *compiler) forExpr(e *syntax.ForExpr) code {
	if rng := c.rangeLoop(e); rng != nil {
		return rng
	}
	iter := c.expr(e.Iter)
	exits := c.mayExit(e.Iter)
	c.fs.push()
	slot := c.declare(e.Var, false, nil).slot
	body := c.block(e.Body)
	c.fs.pop()
	return func(f *frame) Value {
		l := iter(f)
		if exits && f.ctl != ctlNone {
			return nil
		}
		l.(List).Range(func(_ int, x Value) bool {
			f.slots[slot] = x
			body(f)
			switch f.ctl {
			case ctlNone:
				return true
			case ctlBreak:
				f.ctl = ctlNone
				return false
			case ctlContinue:
				f.ctl = ctlNone
				return true
			}
			return false
		})
		if f.ctl != ctlNone {
			return nil
		}
		return Unit
	}
}

// rangeLoop compiles `for i in list.range(a, b)` to a counting loop that
// never builds the list.
func (c *compiler) rangeLoop(e *syntax.ForExpr) code {
	call, ok := e.Iter.(*syntax.CallExpr)
	if !ok || len(call.Args) != 2 {
		return nil
	}
	fi := c.staticFunc(call.Fn)
	if fi == nil || fi.QualName() != "list.range" || c.mayExit(e.Iter) {
		return nil
	}
	pos, _ := placeArgs(call.Args, []string{"start", "stop"})
	var lo, hi icode
	for i, a := range call.Args {
		if pos[i] == 0 {
			lo = c.intExpr(a.Value)
		} else {
			hi = c.intExpr(a.Value)
		}
	}
	if lo == nil || hi == nil {
		return nil
	}
	c.fs.push()
	slot := c.declare(e.Var, false, nil).slot
	body := c.block(e.Body)
	c.fs.pop()
	site := &callSite{name: "list.range", span: call.Span}
	return func(f *frame) Value {
		a, b := lo(f), hi(f)
		if b-a > 50_000_000 {
			f.th.fail("R300", site.span, "list.range: range of %d elements is too large", b-a)
		}
		for i := a; i < b; i++ {
			f.slots[slot] = mkInt(i)
			body(f)
			if f.ctl != ctlNone {
				switch f.ctl {
				case ctlBreak:
					f.ctl = ctlNone
					return Unit
				case ctlContinue:
					f.ctl = ctlNone
					continue
				}
				return nil
			}
		}
		return Unit
	}
}

func (c *compiler) whileExpr(e *syntax.WhileExpr) code {
	cond := c.boolExpr(e.Cond)
	body := c.block(e.Body)
	return func(f *frame) Value {
		for cond(f) {
			body(f)
			if f.ctl != ctlNone {
				if f.ctl == ctlBreak {
					f.ctl = ctlNone
					return Unit
				}
				if f.ctl == ctlContinue {
					f.ctl = ctlNone
					continue
				}
				return nil
			}
		}
		if f.ctl != ctlNone {
			return nil
		}
		return Unit
	}
}

func (c *compiler) returnExpr(e *syntax.ReturnExpr) code {
	if e.X != nil && c.tailOK && c.tailCallee(e.X) != nil {
		// `return f(x)` is a tail call: the call code ends the function itself.
		c.markTailExpr(e.X)
		return c.expr(e.X)
	}
	if e.X == nil {
		return func(f *frame) Value {
			f.ret = Unit
			f.ctl = ctlReturn
			return nil
		}
	}
	x := c.expr(e.X)
	exits := c.mayExit(e.X)
	return func(f *frame) Value {
		v := x(f)
		if exits && f.ctl != ctlNone {
			return nil
		}
		f.ret = v
		f.ctl = ctlReturn
		return nil
	}
}

func (c *compiler) try(e *syntax.TryExpr) code {
	x := c.expr(e.X)
	exits := c.mayExit(e.X)
	prel := c.in.Prog.Prelude.Ctors
	okC, someC := prel["Ok"], prel["Some"]
	return func(f *frame) Value {
		v := x(f)
		if exits && f.ctl != ctlNone {
			return nil
		}
		vv := v.(*Variant)
		if vv.Ctor == okC || vv.Ctor == someC {
			return vv.Fields[0]
		}
		f.ret = vv
		f.ctl = ctlReturn
		return nil
	}
}

// ---- patterns ----

// pattern compiles p to a matcher that binds variables into frame slots. A
// nil matcher matches everything and binds nothing.
func (c *compiler) pattern(p syntax.Pattern) matcher {
	switch p := p.(type) {
	case *syntax.WildPat:
		return nil
	case *syntax.BindPat:
		v := c.declare(p.Name, false, nil)
		s := v.slot
		if p.Name == "_" {
			return nil
		}
		return func(f *frame, x Value) bool { f.slots[s] = x; return true }
	case *syntax.OrPat:
		alts := make([]matcher, len(p.Alts))
		for i, a := range p.Alts {
			alts[i] = c.pattern(a)
			if alts[i] == nil {
				return nil
			}
		}
		return func(f *frame, x Value) bool {
			for _, a := range alts {
				if a(f, x) {
					return true
				}
			}
			return false
		}
	case *syntax.LitPat:
		switch lit := p.Value.(type) {
		case *syntax.IntLit:
			k := lit.Value
			return func(f *frame, x Value) bool { i, ok := x.(int64); return ok && i == k }
		case *syntax.BoolLit:
			k := lit.Value
			return func(f *frame, x Value) bool { b, ok := x.(bool); return ok && b == k }
		case *syntax.StrLit:
			s := ""
			for _, seg := range lit.Segs {
				s += seg.Lit
			}
			return func(f *frame, x Value) bool { t, ok := x.(string); return ok && t == s }
		}
		return func(*frame, Value) bool { return false }
	case *syntax.CtorPat:
		ci := c.ctor(p.Module, p.Name)
		if ci == nil {
			panic("interp: unknown constructor in pattern: " + p.Name)
		}
		type sub struct {
			i int
			m matcher
		}
		var subs []sub
		for i, a := range p.Args {
			if m := c.pattern(a); m != nil {
				subs = append(subs, sub{i, m})
			}
		}
		if len(subs) == 0 {
			return func(f *frame, x Value) bool {
				vv, ok := x.(*Variant)
				return ok && vv.Ctor == ci
			}
		}
		if len(subs) == 1 {
			i, m := subs[0].i, subs[0].m
			return func(f *frame, x Value) bool {
				vv, ok := x.(*Variant)
				return ok && vv.Ctor == ci && m(f, vv.Fields[i])
			}
		}
		return func(f *frame, x Value) bool {
			vv, ok := x.(*Variant)
			if !ok || vv.Ctor != ci {
				return false
			}
			for _, s := range subs {
				if !s.m(f, vv.Fields[s.i]) {
					return false
				}
			}
			return true
		}
	case *syntax.ListPat:
		elems := make([]matcher, len(p.Elems))
		for i, a := range p.Elems {
			elems[i] = c.pattern(a)
		}
		suffix := make([]matcher, len(p.Suffix))
		for i, a := range p.Suffix {
			suffix[i] = c.pattern(a)
		}
		restSlot := -1
		if p.HasRest && p.Rest != "" && p.Rest != "_" {
			restSlot = c.declare(p.Rest, false, nil).slot
		}
		hasRest := p.HasRest
		fixed := len(elems) + len(suffix)
		return func(f *frame, x Value) bool {
			l := x.(List)
			n := l.Len()
			if hasRest {
				if n < fixed {
					return false
				}
			} else if n != fixed {
				return false
			}
			for i, m := range elems {
				if m != nil && !m(f, l.Get(i)) {
					return false
				}
			}
			off := n - len(suffix)
			for i, m := range suffix {
				if m != nil && !m(f, l.Get(off+i)) {
					return false
				}
			}
			if restSlot >= 0 {
				f.slots[restSlot] = l.Slice(len(elems), off)
			}
			return true
		}
	}
	panic("interp: unknown pattern")
}
