package interp

import (
	"github.com/ashraf82de/veld/internal/syntax"
	"github.com/ashraf82de/veld/internal/types"
)

// The compiler turns each function body into a tree of Go closures. Compared
// with walking the AST it resolves every variable to a frame slot, every call
// target and constructor to a pointer, and every field to an index, once,
// instead of on each evaluation. Control flow (`return`, `break`, `continue`,
// `?`) is a flag on the frame that enclosing blocks and loops check, not a
// panic.

type varInfo struct {
	name    string
	slot    int
	boxed   bool
	mutable bool
	// owned marks a list `var` that is updated in place: its vector is held by
	// this slot alone until some read might let it escape (see inplace).
	owned bool
	decl  *syntax.LetStmt
}

// fnScope is the compile-time view of one function or lambda body.
type fnScope struct {
	parent   *fnScope
	blocks   []map[string]*varInfo
	nslots   int
	capSlots []int      // slot in this frame that receives each captured value
	capFrom  []*varInfo // the captured variable, as seen by the parent scope
	capMap   map[*varInfo]*varInfo
}

func newFnScope(parent *fnScope) *fnScope {
	return &fnScope{parent: parent, blocks: []map[string]*varInfo{{}}, capMap: map[*varInfo]*varInfo{}}
}

func (fs *fnScope) newSlot() int {
	s := fs.nslots
	fs.nslots++
	return s
}

func (fs *fnScope) push() { fs.blocks = append(fs.blocks, map[string]*varInfo{}) }
func (fs *fnScope) pop()  { fs.blocks = fs.blocks[:len(fs.blocks)-1] }

func (fs *fnScope) has(name string) bool {
	for s := fs; s != nil; s = s.parent {
		for i := len(s.blocks) - 1; i >= 0; i-- {
			if _, ok := s.blocks[i][name]; ok {
				return true
			}
		}
	}
	return false
}

type compiler struct {
	in       *Interp
	mod      *types.Module
	fs       *fnScope
	boxed    map[*syntax.LetStmt]bool
	owned    map[*syntax.LetStmt]bool
	noFreeze map[*syntax.Ident]bool
	// tailOK says calls in tail position may reuse the caller's Go frame; it is
	// false for functions with `ensures`, which must run after the body.
	tailOK    bool
	tailNodes map[syntax.Expr]bool
	grew      bool
	override  map[syntax.Expr]code
	exits     map[syntax.Node]int
	fnName    string
}

// lookup resolves a variable, capturing it from enclosing function scopes if
// needed. Captured `var`s must be shared between the closure and its creator,
// so they are marked for boxing; the function is then compiled again.
func (c *compiler) lookup(name string) *varInfo { return c.fs.lookup(c, name) }

func (fs *fnScope) lookup(c *compiler, name string) *varInfo {
	for i := len(fs.blocks) - 1; i >= 0; i-- {
		if v := fs.blocks[i][name]; v != nil {
			return v
		}
	}
	if fs.parent == nil {
		return nil
	}
	outer := fs.parent.lookup(c, name)
	if outer == nil {
		return nil
	}
	if p, ok := fs.capMap[outer]; ok {
		return p
	}
	if outer.mutable && outer.decl != nil && !c.boxed[outer.decl] {
		c.boxed[outer.decl] = true
		c.grew = true
	}
	proxy := &varInfo{name: name, slot: fs.newSlot(), boxed: outer.boxed, mutable: outer.mutable, decl: outer.decl}
	fs.capMap[outer] = proxy
	fs.capSlots = append(fs.capSlots, proxy.slot)
	fs.capFrom = append(fs.capFrom, outer)
	return proxy
}

func (c *compiler) declare(name string, mutable bool, decl *syntax.LetStmt) *varInfo {
	v := &varInfo{name: name, slot: c.fs.newSlot(), mutable: mutable, decl: decl}
	if decl != nil && c.boxed[decl] {
		v.boxed = true
	}
	v.owned = decl != nil && decl.Mutable && c.owned[decl] && !v.boxed
	if name != "_" && name != "" {
		c.fs.blocks[len(c.fs.blocks)-1][name] = v
	}
	return v
}

// ---- program ----

func (in *Interp) compileProgram() {
	in.fns = map[*types.FuncInfo]*fnCode{}
	for _, m := range in.Prog.Modules {
		for _, fi := range m.Funcs {
			in.fns[fi] = &fnCode{name: fi.QualName(), fi: fi, nparams: len(fi.Params), effects: fi.Effects}
		}
	}
	for _, m := range in.Prog.Modules {
		for _, fi := range m.Funcs {
			fn := in.fns[fi]
			if fi.Extern {
				name := fi.QualName()
				n := in.natives[name]
				if n == nil {
					n = &Native{Name: name, Fn: func(th *Thread, a []Value) Value {
						th.fail("R999", th.here(), "native function %s is not implemented", name)
						return nil
					}}
				}
				fn.native = n
				continue
			}
			in.compileFn(m, fn)
		}
	}
}

// withBoxing runs build until the set of boxed variables stops growing.
func (in *Interp) withBoxing(mod *types.Module, build func(c *compiler)) {
	boxed := map[*syntax.LetStmt]bool{}
	owned := map[*syntax.LetStmt]bool{}
	for pass := 0; pass < 6; pass++ {
		c := &compiler{in: in, mod: mod, boxed: boxed, owned: owned, noFreeze: map[*syntax.Ident]bool{}, tailNodes: map[syntax.Expr]bool{},
			override: map[syntax.Expr]code{}, exits: map[syntax.Node]int{}}
		build(c)
		if !c.grew {
			return
		}
	}
	panic("interp: variable boxing did not converge")
}

func (in *Interp) compileFn(mod *types.Module, fn *fnCode) {
	d := fn.fi.Decl
	in.withBoxing(mod, func(c *compiler) {
		c.fnName = fn.name
		c.fs = newFnScope(nil)
		for _, p := range d.Params {
			c.declare(p.Name, false, nil)
		}
		fn.requires = nil
		for _, r := range d.Requires {
			fn.requires = append(fn.requires, c.contract(r))
		}
		c.tailOK = len(d.Ensures) == 0
		if c.tailOK {
			c.markTailBlock(d.Body)
		}
		fn.body = c.block(d.Body)
		fn.ensures = nil
		if len(d.Ensures) > 0 {
			c.fs.push()
			fn.resultSlot = c.declare("result", false, nil).slot
			for _, e := range d.Ensures {
				fn.ensures = append(fn.ensures, c.contract(e))
			}
			c.fs.pop()
		}
		fn.nslots = c.fs.nslots
	})
}

func (in *Interp) compileTest(mod *types.Module, t *syntax.TestDecl) *fnCode {
	fn := &fnCode{name: "test " + t.Name}
	in.withBoxing(mod, func(c *compiler) {
		c.fs = newFnScope(nil)
		fn.body = c.block(t.Body)
		fn.nslots = c.fs.nslots
	})
	return fn
}

func (c *compiler) contract(e syntax.Expr) contract {
	ct := contract{test: c.boolExpr(e), expr: e}
	if b, ok := e.(*syntax.BinaryExpr); ok && isCmpOp(b.Op) {
		ct.l, ct.r = c.expr(b.L), c.expr(b.R)
	}
	return ct
}

func isCmpOp(k syntax.Kind) bool {
	switch k {
	case syntax.EQ, syntax.NE, syntax.LT, syntax.LE, syntax.GT, syntax.GE:
		return true
	}
	return false
}

// ---- exit analysis ----

const (
	exitRet  = 1 // `return` or `?`
	exitLoop = 2 // `break` / `continue` not enclosed by a loop inside the node
)

func (c *compiler) mayExit(e syntax.Expr) bool { return c.exitOf(e) != 0 }

func (c *compiler) exitOf(n syntax.Node) int {
	// While a hoisted composite is being compiled its children are already
	// evaluated into temporaries, so they cannot exit; do not memoize then.
	if len(c.override) > 0 {
		if e, ok := n.(syntax.Expr); ok {
			if _, ov := c.override[e]; ov {
				return 0
			}
		}
		return c.exitOf0(n)
	}
	if r, ok := c.exits[n]; ok {
		return r
	}
	r := c.exitOf0(n)
	c.exits[n] = r
	return r
}

func (c *compiler) exitOfAll(xs ...syntax.Expr) int {
	r := 0
	for _, x := range xs {
		if x != nil {
			r |= c.exitOf(x)
		}
	}
	return r
}

func (c *compiler) exitOfBlock(b *syntax.Block) int {
	if b == nil {
		return 0
	}
	r := 0
	for _, s := range b.Stmts {
		switch s := s.(type) {
		case *syntax.LetStmt:
			r |= c.exitOf(s.Value)
		case *syntax.SetStmt:
			r |= c.exitOf(s.Value)
		case *syntax.ExprStmt:
			r |= c.exitOf(s.X)
		case *syntax.ExpectStmt:
			r |= c.exitOf(s.X)
		}
	}
	return r
}

func (c *compiler) exitOf0(n syntax.Node) int {
	switch e := n.(type) {
	case *syntax.ReturnExpr:
		return exitRet | c.exitOfAll(e.X)
	case *syntax.TryExpr:
		return exitRet | c.exitOf(e.X)
	case *syntax.BreakExpr, *syntax.ContinueExpr:
		return exitLoop
	case *syntax.ForExpr:
		return c.exitOf(e.Iter) | c.exitOfBlock(e.Body)&^exitLoop
	case *syntax.WhileExpr:
		return c.exitOf(e.Cond) | c.exitOfBlock(e.Body)&^exitLoop
	case *syntax.IfExpr:
		r := 0
		for _, br := range e.Branches {
			r |= c.exitOf(br.Cond) | c.exitOfBlock(br.Body)
		}
		return r | c.exitOfBlock(e.Else)
	case *syntax.MatchExpr:
		r := c.exitOf(e.X)
		for _, a := range e.Arms {
			r |= c.exitOfAll(a.Guard) | c.exitOfBlock(a.Body)
		}
		return r
	case *syntax.CallExpr:
		r := c.exitOf(e.Fn)
		for _, a := range e.Args {
			r |= c.exitOf(a.Value)
		}
		return r
	case *syntax.PipeExpr:
		return c.exitOf(e.L) | c.exitOf(e.R)
	case *syntax.BinaryExpr:
		return c.exitOf(e.L) | c.exitOf(e.R)
	case *syntax.UnaryExpr:
		return c.exitOf(e.X)
	case *syntax.ParenExpr:
		return c.exitOf(e.X)
	case *syntax.FieldExpr:
		return c.exitOf(e.X)
	case *syntax.ListLit:
		return c.exitOfAll(e.Elems...)
	case *syntax.MapLit:
		r := 0
		for _, en := range e.Entries {
			r |= c.exitOf(en.Key) | c.exitOf(en.Value)
		}
		return r
	case *syntax.RecordLit:
		r := c.exitOfAll(e.Base)
		for _, f := range e.Fields {
			r |= c.exitOf(f.Value)
		}
		return r
	case *syntax.StrLit:
		r := 0
		for _, s := range e.Segs {
			if s.Expr != nil {
				r |= c.exitOf(s.Expr)
			}
		}
		return r
	}
	return 0
}

// ---- blocks and statements ----

func (c *compiler) block(b *syntax.Block) code {
	c.fs.push()
	defer c.fs.pop()
	n := len(b.Stmts)
	stmts := make([]code, n)
	exits := make([]bool, n)
	anyExit := false
	for i, s := range b.Stmts {
		stmts[i] = c.stmt(s)
		exits[i] = c.stmtExits(s)
		anyExit = anyExit || exits[i]
	}
	switch {
	case n == 0:
		return func(*frame) Value { return Unit }
	case n == 1:
		return stmts[0]
	case !anyExit && n == 2:
		s0, s1 := stmts[0], stmts[1]
		return func(f *frame) Value { s0(f); return s1(f) }
	case !anyExit && n == 3:
		s0, s1, s2 := stmts[0], stmts[1], stmts[2]
		return func(f *frame) Value { s0(f); s1(f); return s2(f) }
	case !anyExit:
		last := stmts[n-1]
		init := stmts[:n-1]
		return func(f *frame) Value {
			for _, s := range init {
				s(f)
			}
			return last(f)
		}
	}
	return func(f *frame) Value {
		var last Value = Unit
		for i, s := range stmts {
			last = s(f)
			if exits[i] && f.ctl != ctlNone {
				return nil
			}
		}
		return last
	}
}

func (c *compiler) stmtExits(s syntax.Stmt) bool {
	switch s := s.(type) {
	case *syntax.LetStmt:
		return c.mayExit(s.Value)
	case *syntax.SetStmt:
		return c.mayExit(s.Value)
	case *syntax.ExprStmt:
		return c.mayExit(s.X)
	case *syntax.ExpectStmt:
		return c.mayExit(s.X)
	}
	return false
}

func (c *compiler) stmt(s syntax.Stmt) code {
	switch s := s.(type) {
	case *syntax.LetStmt:
		val := c.expr(s.Value)
		v := c.declare(s.Name, s.Mutable, s)
		slot := v.slot
		exits := c.mayExit(s.Value)
		switch {
		case s.Name == "_":
			return func(f *frame) Value { val(f); return Unit }
		case v.boxed:
			return func(f *frame) Value {
				x := val(f)
				if exits && f.ctl != ctlNone {
					return nil
				}
				f.slots[slot] = &Box{v: x}
				return Unit
			}
		case exits:
			return func(f *frame) Value {
				x := val(f)
				if f.ctl != ctlNone {
					return nil
				}
				f.slots[slot] = x
				return Unit
			}
		}
		return func(f *frame) Value { f.slots[slot] = val(f); return Unit }
	case *syntax.SetStmt:
		if run, ok := c.inplace(s); ok {
			return run
		}
		val := c.expr(s.Value)
		v := c.lookup(s.Name)
		if v == nil {
			panic("interp: assignment to unknown variable " + s.Name)
		}
		slot := v.slot
		exits := c.mayExit(s.Value)
		if v.boxed {
			return func(f *frame) Value {
				x := val(f)
				if exits && f.ctl != ctlNone {
					return nil
				}
				f.slots[slot].(*Box).v = x
				return Unit
			}
		}
		if exits {
			return func(f *frame) Value {
				x := val(f)
				if f.ctl != ctlNone {
					return nil
				}
				f.slots[slot] = x
				return Unit
			}
		}
		return func(f *frame) Value { f.slots[slot] = val(f); return Unit }
	case *syntax.ExprStmt:
		return c.expr(s.X)
	case *syntax.ExpectStmt:
		test := c.boolExpr(s.X)
		var l, r code
		if b, ok := s.X.(*syntax.BinaryExpr); ok && isCmpOp(b.Op) {
			l, r = c.expr(b.L), c.expr(b.R)
		}
		exits := c.mayExit(s.X)
		return func(f *frame) Value {
			ok := test(f)
			if exits && f.ctl != ctlNone {
				return nil
			}
			if !ok {
				th := f.th
				err := &RuntimeError{Code: "T001", Message: "expectation failed: expect " + th.src(s.X), Span: s.X.Sp(), Stack: th.trace()}
				if l != nil {
					err.Details = []string{"left:  " + Repr(l(f)), "right: " + Repr(r(f))}
				}
				panic(err)
			}
			return Unit
		}
	}
	panic("interp: unknown statement")
}
