package types

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/ashraf82de/veld/internal/diag"
	"github.com/ashraf82de/veld/internal/syntax"
)

// Input is one parsed module handed to the checker, in dependency order.
type Input struct {
	Path      string
	File      *syntax.File
	IsStd     bool
	IsPrelude bool
	IsEntry   bool
	Uses      map[string]string // local name -> module path
}

// Program is the result of checking.
type Program struct {
	Modules []*Module
	ByPath  map[string]*Module
	Prelude *Module
}

type local struct {
	name     string
	typ      Type
	mutable  bool
	used     bool
	span     diag.Span
	keyword  diag.Span // span of the `let` keyword, for fixes
	noUnused bool
}

type scope struct {
	vars   map[string]*local
	order  []*local
	parent *scope
}

type hole struct {
	span   diag.Span
	typ    Type
	locals []*local
}

type fnCtx struct {
	name      string
	ret       Type
	effects   EffSet
	collect   *EffSet // non-nil inside a lambda: effects are collected, not checked
	isTest    bool
	loopDepth int
	decl      *syntax.FnDecl
	root      *fnCtx
	holes     []hole
}

// Checker holds state for checking a program.
type Checker struct {
	diags    *diag.List
	nextID   int
	prelude  *Module
	mod      *Module
	fn       *fnCtx
	sc       *scope
	prog     *Program
	suffixID int
	// ExprTypes records the inferred type of every expression.
	ExprTypes map[syntax.Expr]Type
}

var (
	snakeRe  = regexp.MustCompile(`^_?[a-z][a-z0-9_]*$|^_$`)
	pascalRe = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
)

// Check type-checks the given modules (which must be in dependency order,
// prelude first).
func Check(inputs []*Input) (*Program, *diag.List) {
	c := &Checker{diags: &diag.List{}, ExprTypes: map[syntax.Expr]Type{}}
	prog := &Program{ByPath: map[string]*Module{}}
	c.prog = prog
	var mods []*Module
	for _, in := range inputs {
		m := newModule(in.Path, in.File)
		m.IsStd, m.IsPrelude, m.IsEntry = in.IsStd, in.IsPrelude, in.IsEntry
		if m.IsPrelude {
			c.prelude = m
			prog.Prelude = m
		}
		for local, path := range in.Uses {
			if dep, ok := prog.ByPath[path]; ok {
				m.Imports[local] = dep
			} else {
				m.Broken[local] = true
			}
		}
		prog.ByPath[in.Path] = m
		mods = append(mods, m)
		c.declareModule(m)
	}
	for _, m := range mods {
		c.checkModule(m)
	}
	prog.Modules = mods
	return prog, c.diags
}

func (c *Checker) fresh() *TVar {
	c.nextID++
	return &TVar{ID: c.nextID}
}

// poison returns a fresh type for an expression that already has an error.
func (c *Checker) poison() *TVar {
	v := c.fresh()
	v.Poison = true
	return v
}

func (c *Checker) errf(code string, sp diag.Span, format string, args ...any) *diag.Diagnostic {
	return c.diags.Errorf(code, sp, format, args...)
}

// ---------- declarations ----------

func (c *Checker) declareModule(m *Module) {
	c.mod = m
	f := m.File
	seenUse := map[string]bool{}
	for _, u := range f.Uses {
		if seenUse[u.Name()] {
			c.errf("E601", u.Span, "module name `%s` is imported twice", u.Name()).
				Note("two imported modules cannot share a final path segment")
		}
		seenUse[u.Name()] = true
	}
	// 1. type names
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *syntax.TypeDecl:
			c.declareTypeName(m, d.Name, d.TParams, d.Pub, false, d, d.Span)
		case *syntax.RecordDecl:
			c.declareTypeName(m, d.Name, d.TParams, d.Pub, true, d, d.Span)
		}
	}
	// 2. fields and constructors
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *syntax.TypeDecl:
			ti := m.Types[d.Name]
			if ti == nil || ti.Decl != d {
				continue
			}
			env := genEnv(ti.TParams)
			for i, v := range d.Variants {
				c.checkPascal(v.Name, v.Span, "variant")
				ci := &CtorInfo{Name: v.Name, Type: ti, Index: i}
				seen := map[string]bool{}
				for _, fd := range v.Fields {
					c.checkSnake(fd.Name, fd.Span, "field")
					if seen[fd.Name] {
						c.errf("E202", fd.Span, "duplicate field `%s` in variant `%s`", fd.Name, v.Name)
					}
					seen[fd.Name] = true
					ci.Fields = append(ci.Fields, FieldInfo{Name: fd.Name, Type: c.resolveType(fd.Type, env)})
				}
				if prev, ok := m.Ctors[v.Name]; ok {
					c.errf("E202", v.Span, "constructor `%s` is already defined by type `%s`", v.Name, prev.Type.Name)
				} else if c.prelude != nil && m != c.prelude && c.prelude.Ctors[v.Name] != nil {
					c.errf("E202", v.Span, "constructor `%s` conflicts with the built-in constructor of the same name", v.Name)
				}
				m.Ctors[v.Name] = ci
				ti.Ctors = append(ti.Ctors, ci)
			}
		case *syntax.RecordDecl:
			ti := m.Types[d.Name]
			if ti == nil || ti.Decl != d {
				continue
			}
			env := genEnv(ti.TParams)
			seen := map[string]bool{}
			for _, fd := range d.Fields {
				c.checkSnake(fd.Name, fd.Span, "field")
				if seen[fd.Name] {
					c.errf("E202", fd.Span, "duplicate field `%s` in record `%s`", fd.Name, d.Name)
				}
				seen[fd.Name] = true
				ti.Fields = append(ti.Fields, FieldInfo{Name: fd.Name, Type: c.resolveType(fd.Type, env)})
			}
		}
	}
	// 3. function signatures
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *syntax.FnDecl:
			c.checkSnake(d.Name, d.NameSpan, "function")
			if d.Extern && !m.IsStd {
				c.errf("E207", d.Span, "`extern` functions are only allowed in the standard library")
			}
			if prev, ok := m.Funcs[d.Name]; ok {
				c.errf("E202", d.NameSpan, "function `%s` is already defined on line %d", d.Name, prev.Decl.Span.Start.Line)
				continue
			}
			if _, ok := m.Imports[d.Name]; ok {
				c.errf("E202", d.NameSpan, "function `%s` has the same name as an imported module", d.Name)
			}
			fi := &FuncInfo{Name: d.Name, Module: m, Pub: d.Pub, Extern: d.Extern, Decl: d, Effects: EffectsOf(d.Effects)}
			env := map[string]*TGen{}
			for _, tp := range d.TParams {
				c.checkPascal(tp, d.NameSpan, "type parameter")
				c.nextID++
				g := &TGen{Name: tp, ID: c.nextID}
				fi.TParams = append(fi.TParams, g)
				env[tp] = g
			}
			seen := map[string]bool{}
			for _, p := range d.Params {
				c.checkSnake(p.Name, p.Span, "parameter")
				if seen[p.Name] {
					c.errf("E202", p.Span, "duplicate parameter `%s`", p.Name)
				}
				seen[p.Name] = true
				fi.Params = append(fi.Params, ParamInfo{Name: p.Name, Type: c.resolveType(p.Type, env)})
			}
			fi.Ret = c.resolveType(d.Ret, env)
			m.Funcs[d.Name] = fi
		case *syntax.TestDecl:
			m.Tests = append(m.Tests, d)
		}
	}
}

func genEnv(gs []*TGen) map[string]*TGen {
	env := map[string]*TGen{}
	for _, g := range gs {
		env[g.Name] = g
	}
	return env
}

func (c *Checker) declareTypeName(m *Module, name string, tparams []string, pub, isRecord bool, d syntax.Decl, sp diag.Span) {
	c.checkPascal(name, sp, "type")
	if isBuiltinName(name) || (c.prelude != nil && m != c.prelude && c.prelude.Types[name] != nil) {
		c.errf("E202", sp, "type `%s` conflicts with a built-in type", name)
		return
	}
	if _, ok := m.Types[name]; ok {
		c.errf("E202", sp, "type `%s` is already defined", name)
		return
	}
	ti := &TypeInfo{Name: name, Module: m, Pub: pub, IsRecord: isRecord, Decl: d}
	for _, tp := range tparams {
		c.nextID++
		ti.TParams = append(ti.TParams, &TGen{Name: tp, ID: c.nextID})
	}
	m.Types[name] = ti
}

func (c *Checker) checkSnake(name string, sp diag.Span, what string) {
	if !snakeRe.MatchString(name) {
		d := c.errf("E201", sp, "%s name `%s` must be snake_case", what, name)
		if s := toSnake(name); s != name && snakeRe.MatchString(s) {
			d.Note("rename it to `%s`", s)
		}
	}
}

func (c *Checker) checkPascal(name string, sp diag.Span, what string) {
	if !pascalRe.MatchString(name) {
		c.errf("E201", sp, "%s name `%s` must be PascalCase", what, name)
	}
}

func toSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r + 32)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var typeHints = map[string]string{
	"String": "Str", "string": "Str", "str": "Str", "int": "Int", "Integer": "Int", "i64": "Int", "i32": "Int",
	"float": "Float", "Double": "Float", "double": "Float", "f64": "Float", "bool": "Bool", "Boolean": "Bool",
	"Void": "Unit", "void": "Unit", "None": "Unit", "Array": "List", "Vec": "List", "list": "List",
	"Dict": "Map", "HashMap": "Map", "dict": "Map", "Maybe": "Option", "Optional": "Option", "Either": "Result",
}

func (c *Checker) resolveType(te syntax.TypeExpr, env map[string]*TGen) Type {
	switch te := te.(type) {
	case nil:
		return c.fresh()
	case *syntax.FnType:
		ps := make([]Type, len(te.Params))
		for i, p := range te.Params {
			ps[i] = c.resolveType(p, env)
		}
		return &TFn{Params: ps, Ret: c.resolveType(te.Ret, env), Effects: EffectsOf(te.Effects)}
	case *syntax.NamedType:
		args := make([]Type, len(te.Args))
		for i, a := range te.Args {
			args[i] = c.resolveType(a, env)
		}
		arity := func(n int) bool {
			if len(args) != n {
				c.errf("E302", te.Span, "type `%s` takes %d type argument(s), found %d", te.Name, n, len(args))
				return false
			}
			return true
		}
		if te.Module == "" {
			if g, ok := env[te.Name]; ok {
				if len(args) > 0 {
					c.errf("E302", te.Span, "type parameter `%s` cannot take type arguments", te.Name)
				}
				return g
			}
			switch te.Name {
			case "Int", "Float", "Bool", "Str", "Unit":
				if arity(0) {
					return &TCon{Name: te.Name}
				}
				return c.fresh()
			case "List":
				if arity(1) {
					return ListOf(args[0])
				}
				return c.fresh()
			case "Map":
				if arity(2) {
					return MapOf(args[0], args[1])
				}
				return c.fresh()
			}
			ti := c.mod.Types[te.Name]
			if ti == nil && c.prelude != nil {
				ti = c.prelude.Types[te.Name]
			}
			if ti == nil {
				d := c.errf("E203", te.Span, "unknown type `%s`", te.Name)
				if h, ok := typeHints[te.Name]; ok {
					d.WithFix("use `"+h+"`", nameSpan(te), h)
				} else if s := diag.Suggest(te.Name, c.typeNames(env)); s != "" {
					d.WithFix("did you mean `"+s+"`?", nameSpan(te), s)
				}
				return c.fresh()
			}
			if arity(len(ti.TParams)) {
				return &TCon{Name: ti.QualName(), Info: ti, Args: args}
			}
			return c.fresh()
		}
		dep, ok := c.mod.Imports[te.Module]
		if !ok {
			c.errf("E601", te.Span, "unknown module `%s`", te.Module).Note("import it with `use`")
			return c.fresh()
		}
		ti := dep.Types[te.Name]
		if ti == nil {
			d := c.errf("E203", te.Span, "module `%s` has no type `%s`", te.Module, te.Name)
			if s := diag.Suggest(te.Name, keys(dep.Types)); s != "" {
				d.Note("did you mean `%s.%s`?", te.Module, s)
			}
			return c.fresh()
		}
		if !ti.Pub {
			c.errf("E206", te.Span, "type `%s.%s` is not `pub`", te.Module, te.Name)
		}
		if arity(len(ti.TParams)) {
			return &TCon{Name: ti.QualName(), Info: ti, Args: args}
		}
		return c.fresh()
	}
	return c.fresh()
}

func nameSpan(te *syntax.NamedType) diag.Span {
	sp := te.Span
	sp.End = sp.Start
	sp.End.Col += len(te.Name)
	sp.End.Offset += len(te.Name)
	return sp
}

func (c *Checker) typeNames(env map[string]*TGen) []string {
	out := []string{"Int", "Float", "Bool", "Str", "Unit", "List", "Map"}
	out = append(out, keys(c.mod.Types)...)
	if c.prelude != nil {
		out = append(out, keys(c.prelude.Types)...)
	}
	for k := range env {
		out = append(out, k)
	}
	return out
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ---------- bodies ----------

func (c *Checker) checkModule(m *Module) {
	c.mod = m
	for _, d := range m.File.Decls {
		switch d := d.(type) {
		case *syntax.FnDecl:
			fi := m.Funcs[d.Name]
			if fi == nil || fi.Decl != d || d.Extern {
				continue
			}
			c.checkFn(fi)
		case *syntax.TestDecl:
			c.fn = &fnCtx{name: "test", ret: TUnit, effects: AllEffects, isTest: true}
			c.fn.root = c.fn
			c.sc = &scope{vars: map[string]*local{}}
			c.checkBlock(d.Body, nil)
			c.popScopeWarn(c.sc)
			c.reportHoles()
		}
	}
}

func (c *Checker) checkFn(fi *FuncInfo) {
	d := fi.Decl
	c.fn = &fnCtx{name: fi.Name, ret: fi.Ret, effects: fi.Effects, decl: d}
	c.fn.root = c.fn
	c.sc = &scope{vars: map[string]*local{}}
	for i, p := range d.Params {
		c.declare(p.Name, fi.Params[i].Type, false, p.Span, diag.Span{}, true)
	}
	for _, r := range d.Requires {
		c.expectType(r, TBool, "a `requires` clause")
	}
	unitRet := isCon(fi.Ret, "Unit")
	if unitRet {
		c.checkBlock(d.Body, nil)
	} else {
		c.checkBlock(d.Body, fi.Ret)
	}
	if d.Ret == nil {
		t := Prune(fi.Ret)
		if isUnresolved(t) {
			Unify(t, TUnit)
		}
		shown := Show(t)
		pos := d.HeadEnd
		c.errf("E108", d.NameSpan, "function `%s` needs a return type (inferred: %s)", d.Name, shown).
			WithFix("declare `-> "+shown+"`", diag.Span{File: d.NameSpan.File, Start: pos, End: pos}, " -> "+shown).
			Note("every function declares its return type, even Unit")
	}
	if len(d.Ensures) > 0 {
		c.pushScope()
		c.declare("result", fi.Ret, false, d.NameSpan, diag.Span{}, true)
		for _, e := range d.Ensures {
			c.expectType(e, TBool, "an `ensures` clause")
		}
		c.popScope()
	}
	c.popScopeWarn(c.sc)
	c.reportHoles()
}

func (c *Checker) reportHoles() {
	for _, h := range c.fn.holes {
		d := c.diags.Infof("H001", h.span, "hole `???` has type %s", Show(h.typ))
		var parts []string
		for _, l := range h.locals {
			parts = append(parts, l.name+": "+Show(l.typ))
		}
		if len(parts) > 0 {
			d.Note("in scope: %s", strings.Join(parts, ", "))
		}
	}
}

func (c *Checker) pushScope() { c.sc = &scope{vars: map[string]*local{}, parent: c.sc} }
func (c *Checker) popScope() {
	c.popScopeWarn(c.sc)
	c.sc = c.sc.parent
}

func (c *Checker) popScopeWarn(s *scope) {
	for _, l := range s.order {
		if !l.used && !l.noUnused && !strings.HasPrefix(l.name, "_") {
			c.diags.Warnf("W201", l.span, "`%s` is never used", l.name).
				WithFix("prefix it with `_` to mark it intentionally unused", diag.Span{File: l.span.File, Start: l.span.Start, End: l.span.Start}, "_")
		}
	}
}

func (c *Checker) lookupLocal(name string) *local {
	for s := c.sc; s != nil; s = s.parent {
		if l, ok := s.vars[name]; ok {
			return l
		}
	}
	return nil
}

func (c *Checker) declare(name string, t Type, mutable bool, sp, kw diag.Span, isParam bool) {
	if name == "_" {
		return
	}
	c.checkSnake(name, sp, "variable")
	if prev := c.lookupLocal(name); prev != nil {
		c.errf("E204", sp, "`%s` is already bound in this function (line %d); Veld does not allow shadowing", name, prev.span.Start.Line).
			Note("choose a new name, or use `var` + `set` to update a value")
	} else if _, ok := c.mod.Imports[name]; ok {
		c.errf("E204", sp, "`%s` is the name of an imported module", name)
	}
	l := &local{name: name, typ: t, mutable: mutable, span: sp, keyword: kw, noUnused: isParam}
	c.sc.vars[name] = l
	c.sc.order = append(c.sc.order, l)
}

func (c *Checker) visibleLocals() []*local {
	var out []*local
	seen := map[string]bool{}
	for s := c.sc; s != nil; s = s.parent {
		for i := len(s.order) - 1; i >= 0; i-- {
			l := s.order[i]
			if !seen[l.name] {
				seen[l.name] = true
				out = append(out, l)
			}
		}
	}
	if len(out) > 12 {
		out = out[:12]
	}
	return out
}

// require records that the current code performs effects eff.
func (c *Checker) require(eff EffSet, sp diag.Span, what string) {
	if eff == 0 {
		return
	}
	if c.fn.collect != nil {
		*c.fn.collect |= eff
		return
	}
	missing := eff &^ c.fn.effects
	if missing == 0 {
		return
	}
	d := c.errf("E401", sp, "%s uses effect `%s`, but function `%s` does not declare it", what, missing.String(), c.fn.name)
	if c.fn.decl != nil {
		pos := c.fn.decl.HeadEnd
		all := (c.fn.effects | missing).Names()
		if c.fn.effects == 0 {
			d.WithFix("declare the effect", diag.Span{File: sp.File, Start: pos, End: pos}, " uses "+strings.Join(missing.Names(), ", "))
		} else {
			d.WithFix("declare the effect", diag.Span{File: sp.File, Start: pos, End: pos}, ", "+strings.Join(missing.Names(), ", "))
		}
		d.Note("the full clause would be: uses %s", strings.Join(all, ", "))
		d.Note("effects propagate: every caller of `%s` must declare them too", c.fn.name)
	}
}

// ---------- blocks and statements ----------

// checkBlock checks stmts. If want is nil the block's value is discarded;
// otherwise the final expression must have type want.
func (c *Checker) checkBlock(b *syntax.Block, want Type) Type {
	c.pushScope()
	defer c.popScope()
	if want == nil {
		for _, s := range b.Stmts {
			c.checkStmt(s)
		}
		return TUnit
	}
	if len(b.Stmts) == 0 {
		if !Unify(want, TUnit) {
			c.errf("E301", b.Span, "empty block has type Unit, but %s is expected", Show(want))
		}
		return want
	}
	for _, s := range b.Stmts[:len(b.Stmts)-1] {
		c.checkStmt(s)
	}
	last := b.Stmts[len(b.Stmts)-1]
	es, ok := last.(*syntax.ExprStmt)
	if !ok {
		c.checkStmt(last)
		if !isCon(want, "Unit") && !Unify(want, TUnit) {
			c.errf("E305", last.Sp(), "this block must end with an expression of type %s", Show(want)).
				Note("the last line of a block is its value")
		}
		return want
	}
	t := c.checkExpr(es.X, want)
	c.unifyAt(want, t, es.X.Sp(), "block value")
	return want
}

func (c *Checker) checkStmt(s syntax.Stmt) {
	switch s := s.(type) {
	case *syntax.LetStmt:
		var t Type
		if s.Type != nil {
			t = c.resolveType(s.Type, c.genericEnv())
			vt := c.checkExpr(s.Value, t)
			c.unifyAt(t, vt, s.Value.Sp(), "binding `"+s.Name+"`")
		} else {
			t = c.checkExpr(s.Value, nil)
			if isCon(t, "Unit") {
				c.diags.Warnf("W202", s.Span, "`%s` is bound to a Unit value", s.Name)
			}
		}
		kw := s.Span
		kw.End = kw.Start
		kw.End.Col += 3
		kw.End.Offset += 3
		c.declare(s.Name, t, s.Mutable, s.NameSpan, kw, false)
	case *syntax.SetStmt:
		l := c.lookupLocal(s.Name)
		if l == nil {
			c.unknownName(s.Name, s.NameSpan)
			c.checkExpr(s.Value, nil)
			return
		}
		if !l.mutable {
			d := c.errf("E306", s.NameSpan, "cannot `set` `%s`: it was declared with `let`", s.Name)
			if !l.keyword.IsZero() {
				d.WithFix("declare it with `var`", l.keyword, "var")
			} else {
				d.Note("parameters and pattern bindings are immutable; copy into a `var` first")
			}
		}
		t := c.checkExpr(s.Value, l.typ)
		c.unifyAt(l.typ, t, s.Value.Sp(), "assignment to `"+s.Name+"`")
		l.used = true
	case *syntax.ExpectStmt:
		if !c.fn.root.isTest {
			c.errf("E307", s.Span, "`expect` can only be used inside a `test` block").
				Note("use `requires`/`ensures` contracts or return a Result for runtime checks")
		}
		c.expectType(s.X, TBool, "`expect`")
	case *syntax.ExprStmt:
		t := c.checkDiscard(s.X)
		switch x := s.X.(type) {
		case *syntax.IfExpr, *syntax.MatchExpr, *syntax.ForExpr, *syntax.WhileExpr, *syntax.ReturnExpr,
			*syntax.BreakExpr, *syntax.ContinueExpr, *syntax.TodoExpr:
		default:
			pt := Prune(t)
			if _, isVar := pt.(*TVar); !isVar && !isCon(pt, "Unit") {
				d := c.errf("E308", s.Span, "unused value of type %s", Show(pt))
				if call := callOf(x); call != nil {
					d.Note("Veld values are immutable: functions like list.push return a new value instead of changing their argument")
					d.WithFix("bind it with `let`", diag.Span{File: s.Span.File, Start: s.Span.Start, End: s.Span.Start}, "let _ = ")
				}
			}
		}
	}
}

func callOf(e syntax.Expr) *syntax.CallExpr {
	switch e := e.(type) {
	case *syntax.CallExpr:
		return e
	case *syntax.PipeExpr:
		return e.R
	case *syntax.TryExpr:
		return callOf(e.X)
	}
	return nil
}

func (c *Checker) genericEnv() map[string]*TGen {
	env := map[string]*TGen{}
	if c.fn != nil && c.fn.root.decl != nil {
		if fi := c.mod.Funcs[c.fn.root.decl.Name]; fi != nil {
			for _, g := range fi.TParams {
				env[g.Name] = g
			}
		}
	}
	return env
}

func (c *Checker) expectType(e syntax.Expr, want Type, what string) {
	t := c.checkExpr(e, want)
	c.unifyAt(want, t, e.Sp(), what)
}

// unifyAt unifies and reports a mismatch with targeted hints.
func (c *Checker) unifyAt(want, got Type, sp diag.Span, what string) bool {
	if Unify(want, got) {
		return true
	}
	c.mismatch(want, got, sp, what)
	return false
}

func (c *Checker) mismatch(want, got Type, sp diag.Span, what string) {
	if isPoison(want) || isPoison(got) {
		return
	}
	w, g := Prune(want), Prune(got)
	d := c.errf("E301", sp, "type mismatch in %s: expected %s, found %s", what, Show(w), Show(g))
	switch {
	case isCon(w, "Float") && isCon(g, "Int"):
		d.Note("Int does not convert implicitly; use math.to_float(x) or write a float literal like 1.0")
	case isCon(w, "Int") && isCon(g, "Float"):
		d.Note("use math.round(x), math.floor(x) or math.ceil(x) to convert Float to Int")
	case isCon(w, "Str") && !isCon(g, "Str"):
		d.Note("convert any value to Str with interpolation: \"${x}\"")
	case isOpt(w) && !isOpt(g):
		d.Note("wrap the value: Some(x)")
	case isOpt(g) && !isOpt(w):
		d.Note("this is an Option; handle both cases with `match`, unwrap with `?`, or use option.unwrap_or(x, default)")
	case isRes(g) && !isRes(w):
		d.Note("this is a Result; handle it with `match` or propagate the error with `?`")
	}
}

func isOpt(t Type) bool {
	c, ok := Prune(t).(*TCon)
	return ok && c.Info != nil && c.Info.Name == "Option" && c.Info.Module.IsPrelude
}

func isRes(t Type) bool {
	c, ok := Prune(t).(*TCon)
	return ok && c.Info != nil && c.Info.Name == "Result" && c.Info.Module.IsPrelude
}

var fmtT = fmt.Sprintf
