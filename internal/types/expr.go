package types

import (
	"strconv"
	"strings"

	"github.com/ashraf82de/veld/internal/diag"
	"github.com/ashraf82de/veld/internal/syntax"
	"github.com/ashraf82de/veld/std"
)

var foreignHints = map[string]string{
	"print": "io.print(s) (add `use std.io`)", "println": "io.print(s) (add `use std.io`)", "printf": "io.print(\"...${x}\")",
	"len": "list.len(xs) or str.len(s)", "length": "list.len(xs) or str.len(s)", "range": "list.range(start, stop)",
	"str": "string interpolation \"${x}\"", "to_string": "string interpolation \"${x}\"", "string": "string interpolation \"${x}\"",
	"int": "str.to_int(s) or math.round(f)", "parse_int": "str.to_int(s)", "float": "math.to_float(i) or str.to_float(s)",
	"map": "list.map(xs, f)", "filter": "list.filter(xs, f)", "reduce": "list.fold(xs, init, f)", "sorted": "list.sort(xs)",
	"sum": "list.sum(xs)", "abs": "math.abs(x)", "min": "math.min(a, b)", "max": "math.max(a, b)",
	"input": "io.read_line()", "open": "fs.read(path)", "null": "None", "nil": "None", "assert": "`expect` inside a test",
	"append": "list.push(xs, x)", "push": "list.push(xs, x)", "enumerate": "list.enumerate(xs)",
	"zip": "list.zip(a, b)", "join": "str.join(parts, sep)", "split": "str.split(s, sep)",
	"exit": "env.exit(code)", "sqrt": "math.sqrt(x)", "floor": "math.floor(x)", "round": "math.round(x)",
	"unwrap": "option.unwrap_or(x, default) or `?`", "throw": "return Err(...)", "raise": "return Err(...)",
}

// checkExpr infers the type of e. want, if non-nil, is the type expected by
// the context and is used to guide inference (e.g. lambda parameters).
//
// A nil want still means the value is used; statement-level expressions whose
// value is discarded go through checkDiscard instead.
func (c *Checker) checkExpr(e syntax.Expr, want Type) Type {
	if want == nil {
		switch e.(type) {
		case *syntax.IfExpr, *syntax.MatchExpr:
			want = c.fresh()
		}
	}
	t := c.inferExpr(e, want)
	c.ExprTypes[e] = t
	return t
}

func (c *Checker) inferExpr(e syntax.Expr, want Type) Type {
	switch e := e.(type) {
	case *syntax.IntLit:
		return TInt
	case *syntax.FloatLit:
		return TFloat
	case *syntax.BoolLit:
		return TBool
	case *syntax.StrLit:
		for _, s := range e.Segs {
			if s.Expr != nil {
				t := c.checkExpr(s.Expr, nil)
				if _, isFn := Prune(t).(*TFn); isFn {
					c.errf("E309", s.Expr.Sp(), "cannot interpolate a function into a string")
				}
			}
		}
		return TStr
	case *syntax.ParenExpr:
		return c.checkExpr(e.X, want)
	case *syntax.Ident:
		return c.checkIdent(e)
	case *syntax.FieldExpr:
		return c.checkField(e)
	case *syntax.CallExpr:
		return c.checkCall(e, nil, want)
	case *syntax.PipeExpr:
		return c.checkCall(e.R, e.L, want)
	case *syntax.BinaryExpr:
		return c.checkBinary(e)
	case *syntax.UnaryExpr:
		if e.Op == syntax.KW_NOT {
			c.expectType(e.X, TBool, "`not` operand")
			return TBool
		}
		t := c.checkExpr(e.X, want)
		if !isCon(t, "Int") && !isCon(t, "Float") {
			c.errf("E310", e.Span, "unary `-` needs Int or Float, found %s", Show(t))
		}
		return t
	case *syntax.ListLit:
		elem := Type(c.fresh())
		if w, ok := Prune(want).(*TCon); ok && w.Info == nil && w.Name == "List" {
			elem = w.Args[0]
		}
		for _, x := range e.Elems {
			t := c.checkExpr(x, elem)
			c.unifyAt(elem, t, x.Sp(), "list element")
		}
		return ListOf(elem)
	case *syntax.MapLit:
		var k, v Type = c.fresh(), c.fresh()
		if w, ok := Prune(want).(*TCon); ok && w.Info == nil && w.Name == "Map" {
			k, v = w.Args[0], w.Args[1]
		}
		for _, en := range e.Entries {
			c.unifyAt(k, c.checkExpr(en.Key, k), en.Key.Sp(), "map key")
			c.unifyAt(v, c.checkExpr(en.Value, v), en.Value.Sp(), "map value")
		}
		if containsFn(k) {
			c.errf("E311", e.Span, "functions cannot be map keys")
		}
		return MapOf(k, v)
	case *syntax.RecordLit:
		return c.checkRecordLit(e)
	case *syntax.LambdaExpr:
		return c.checkLambda(e, want)
	case *syntax.IfExpr:
		return c.checkIf(e, want)
	case *syntax.MatchExpr:
		return c.checkMatch(e, want)
	case *syntax.ForExpr:
		it := c.checkExpr(e.Iter, nil)
		elem := Type(c.fresh())
		if !Unify(it, ListOf(elem)) {
			d := c.errf("E312", e.Iter.Sp(), "`for` iterates over a List, found %s", Show(it))
			switch {
			case isCon(it, "Map"):
				d.Note("iterate the keys: for k in map.keys(m)")
			case isCon(it, "Str"):
				d.Note("iterate the characters: for ch in str.chars(s)")
			case isCon(it, "Int"):
				d.Note("iterate a range: for i in list.range(0, n)")
			}
		}
		c.pushScope()
		c.declare(e.Var, elem, false, e.VarSpan, diag.Span{}, false)
		c.fn.loopDepth++
		c.checkBlock(e.Body, nil)
		c.fn.loopDepth--
		c.popScope()
		return TUnit
	case *syntax.WhileExpr:
		c.expectType(e.Cond, TBool, "`while` condition")
		c.fn.loopDepth++
		c.checkBlock(e.Body, nil)
		c.fn.loopDepth--
		return TUnit
	case *syntax.ReturnExpr:
		if e.X == nil {
			if !Unify(c.fn.ret, TUnit) {
				c.errf("E313", e.Span, "`return` without a value in a function returning %s", Show(c.fn.ret))
			}
		} else {
			t := c.checkExpr(e.X, c.fn.ret)
			c.unifyAt(c.fn.ret, t, e.X.Sp(), "return value")
		}
		return c.fresh()
	case *syntax.BreakExpr, *syntax.ContinueExpr:
		if c.fn.loopDepth == 0 {
			c.errf("E314", e.Sp(), "`break`/`continue` outside of a loop")
		}
		return c.fresh()
	case *syntax.TodoExpr:
		t := want
		if t == nil {
			t = c.fresh()
		}
		c.fn.root.holes = append(c.fn.root.holes, hole{span: e.Span, typ: t, locals: c.visibleLocals()})
		return t
	case *syntax.TryExpr:
		return c.checkTry(e)
	}
	c.errf("E399", e.Sp(), "unsupported expression")
	return c.fresh()
}

// checkDiscard checks an expression statement whose value is not used.
func (c *Checker) checkDiscard(e syntax.Expr) Type {
	var t Type
	switch x := e.(type) {
	case *syntax.IfExpr:
		t = c.checkIf(x, nil)
	case *syntax.MatchExpr:
		t = c.checkMatch(x, nil)
	default:
		return c.checkExpr(e, nil)
	}
	c.ExprTypes[e] = t
	return t
}

// ---- names ----

func (c *Checker) lookupFunc(name string) *FuncInfo {
	if f := c.mod.Funcs[name]; f != nil {
		return f
	}
	if c.prelude != nil {
		if f := c.prelude.Funcs[name]; f != nil && f.Pub {
			return f
		}
	}
	return nil
}

func (c *Checker) lookupCtor(module, name string, sp diag.Span) *CtorInfo {
	if module != "" {
		dep, ok := c.mod.Imports[module]
		if !ok {
			c.unknownModule(module, sp)
			return nil
		}
		ci := dep.Ctors[name]
		if ci == nil {
			d := c.errf("E205", sp, "module `%s` has no constructor `%s`", module, name)
			if s := diag.Suggest(name, keys(dep.Ctors)); s != "" {
				d.Note("did you mean `%s.%s`?", module, s)
			}
			return nil
		}
		if !ci.Type.Pub {
			c.errf("E206", sp, "type `%s.%s` is not `pub`", module, ci.Type.Name)
		}
		return ci
	}
	if ci := c.mod.Ctors[name]; ci != nil {
		return ci
	}
	if c.prelude != nil {
		if ci := c.prelude.Ctors[name]; ci != nil {
			return ci
		}
	}
	d := c.errf("E205", sp, "unknown constructor `%s`", name)
	var cands []string
	cands = append(cands, keys(c.mod.Ctors)...)
	if c.prelude != nil {
		cands = append(cands, keys(c.prelude.Ctors)...)
	}
	if s := diag.Suggest(name, cands); s != "" {
		d.WithFix("did you mean `"+s+"`?", ctorNameSpan(sp, name), s)
	}
	if ti := c.mod.Types[name]; ti != nil && ti.IsRecord {
		d.Note("`%s` is a record; construct it with `%s{field: value}`", name, name)
	}
	return nil
}

func ctorNameSpan(sp diag.Span, name string) diag.Span {
	sp.End = sp.Start
	sp.End.Col += len(name)
	sp.End.Offset += len(name)
	return sp
}

func (c *Checker) unknownModule(name string, sp diag.Span) {
	d := c.errf("E601", sp, "unknown module `%s`", name)
	if stdModules[name] {
		d.WithFix("import it", diag.Span{File: sp.File, Start: diag.Pos{Line: 1, Col: 1}, End: diag.Pos{Line: 1, Col: 1}}, "use std."+name+"\n")
	} else {
		d.Note("import it with `use path.to.%s`", name)
	}
}

// stdModules lists the embedded standard library modules, so that a missing
// `use` gets an "import it" fix for every one of them.
var stdModules = func() map[string]bool {
	out := map[string]bool{}
	entries, _ := std.FS.ReadDir(".")
	for _, e := range entries {
		if name, ok := strings.CutSuffix(e.Name(), ".veld"); ok && name != "prelude" {
			out[name] = true
		}
	}
	return out
}()

func (c *Checker) unknownName(name string, sp diag.Span) {
	d := c.errf("E205", sp, "unknown name `%s`", name)
	var cands []string
	for s := c.sc; s != nil; s = s.parent {
		cands = append(cands, keys(s.vars)...)
	}
	cands = append(cands, keys(c.mod.Funcs)...)
	if c.prelude != nil {
		cands = append(cands, keys(c.prelude.Funcs)...)
	}
	if s := diag.Suggest(name, cands); s != "" {
		d.WithFix("did you mean `"+s+"`?", sp, s)
	} else if h, ok := foreignHints[name]; ok {
		d.Note("Veld equivalent: %s", h)
	}
}

func (c *Checker) instantiateFn(fi *FuncInfo) *TFn {
	m := map[int]Type{}
	for _, g := range fi.TParams {
		m[g.ID] = c.fresh()
	}
	ps := make([]Type, len(fi.Params))
	for i, p := range fi.Params {
		ps[i] = Subst(p.Type, m)
	}
	return &TFn{Params: ps, Ret: Subst(fi.Ret, m), Effects: fi.Effects}
}

func (c *Checker) ctorValue(ci *CtorInfo, sp diag.Span) Type {
	t, _ := c.instantiateType(ci.Type)
	if len(ci.Fields) > 0 {
		var names []string
		for _, f := range ci.Fields {
			names = append(names, f.Name)
		}
		c.errf("E315", sp, "constructor `%s` has fields (%s); call it: %s(...)", ci.Name, strings.Join(names, ", "), ci.Name)
	}
	return t
}

func (c *Checker) checkIdent(e *syntax.Ident) Type {
	if l := c.lookupLocal(e.Name); l != nil {
		l.used = true
		return l.typ
	}
	if e.Name == "Unit" {
		return TUnit
	}
	if syntax.IsUpper(e.Name) {
		if ci := c.lookupCtor("", e.Name, e.Span); ci != nil {
			return c.ctorValue(ci, e.Span)
		}
		return c.fresh()
	}
	if fi := c.lookupFunc(e.Name); fi != nil {
		return c.instantiateFn(fi)
	}
	if _, ok := c.mod.Imports[e.Name]; ok {
		c.errf("E316", e.Span, "module `%s` cannot be used as a value", e.Name).Note("access a member: %s.name", e.Name)
		return c.fresh()
	}
	if e.Name == "result" && c.fn.decl != nil {
		c.errf("E205", e.Span, "`result` is only available inside `ensures` clauses")
		return c.fresh()
	}
	c.unknownName(e.Name, e.Span)
	return c.poison()
}

// moduleMember resolves `mod.name` when X names an imported module.
func (c *Checker) moduleOf(x syntax.Expr) (*Module, string, bool) {
	id, ok := x.(*syntax.Ident)
	if !ok || c.lookupLocal(id.Name) != nil {
		return nil, "", false
	}
	if dep, ok := c.mod.Imports[id.Name]; ok {
		return dep, id.Name, true
	}
	if c.mod.Broken[id.Name] {
		return nil, id.Name, true
	}
	if stdModules[id.Name] && c.mod.Funcs[id.Name] == nil {
		c.unknownModule(id.Name, id.Span)
		return nil, id.Name, true
	}
	return nil, "", false
}

func (c *Checker) memberFunc(dep *Module, alias, name string, sp diag.Span) *FuncInfo {
	fi := dep.Funcs[name]
	if fi == nil {
		d := c.errf("E205", sp, "module `%s` has no function `%s`", alias, name)
		var pubs []string
		for n, f := range dep.Funcs {
			if f.Pub {
				pubs = append(pubs, n)
			}
		}
		if s := diag.Suggest(name, pubs); s != "" {
			d.WithFix("did you mean `"+alias+"."+s+"`?", sp, s)
		} else {
			d.Note("run `veld describe std.%s` to list its functions", alias)
		}
		return nil
	}
	if !fi.Pub {
		c.errf("E206", sp, "function `%s.%s` is not `pub`", alias, name)
	}
	return fi
}

func (c *Checker) checkField(e *syntax.FieldExpr) Type {
	if dep, alias, ok := c.moduleOf(e.X); ok {
		if dep == nil {
			return c.poison()
		}
		if syntax.IsUpper(e.Name) {
			if ci := c.lookupCtor(alias, e.Name, e.NameSpan); ci != nil {
				return c.ctorValue(ci, e.Span)
			}
			return c.poison()
		}
		if fi := c.memberFunc(dep, alias, e.Name, e.NameSpan); fi != nil {
			return c.instantiateFn(fi)
		}
		return c.poison()
	}
	xt := Prune(c.checkExpr(e.X, nil))
	switch t := xt.(type) {
	case *TCon:
		if t.Info != nil && t.Info.IsRecord {
			f, _, ok := t.Info.Field(e.Name)
			if !ok {
				d := c.errf("E317", e.NameSpan, "record `%s` has no field `%s`", t.Info.Name, e.Name)
				var names []string
				for _, f := range t.Info.Fields {
					names = append(names, f.Name)
				}
				if s := diag.Suggest(e.Name, names); s != "" {
					d.WithFix("did you mean `"+s+"`?", e.NameSpan, s)
				} else {
					d.Note("fields are: %s", strings.Join(names, ", "))
				}
				return c.poison()
			}
			m := map[int]Type{}
			for i, g := range t.Info.TParams {
				m[g.ID] = t.Args[i]
			}
			return Subst(f.Type, m)
		}
		d := c.errf("E317", e.NameSpan, "type %s has no field `%s`", Show(t), e.Name)
		mod := ""
		switch {
		case t.Info == nil && t.Name == "List":
			mod = "list"
		case t.Info == nil && t.Name == "Str":
			mod = "str"
		case t.Info == nil && t.Name == "Map":
			mod = "map"
		case isOpt(t):
			mod = "option"
		case isRes(t):
			mod = "result"
		}
		if mod != "" {
			d.Note("Veld has no methods; call %s.%s(x, ...) (with `use std.%s`) or pipe: x |> %s.%s()", mod, e.Name, mod, mod, e.Name)
		} else if t.Info != nil && !t.Info.IsRecord {
			d.Note("`%s` is a sum type; use `match` to access variant fields", t.Info.Name)
		}
		return c.poison()
	case *TVar:
		c.errf("E318", e.X.Sp(), "cannot infer the type of this expression before accessing `.%s`", e.Name).
			Note("add a type annotation to the parameter or binding")
		return c.poison()
	default:
		c.errf("E317", e.NameSpan, "type %s has no field `%s`", Show(xt), e.Name)
		return c.poison()
	}
}

// ---- calls ----

type callee struct {
	name    string
	params  []ParamInfo // declared (for names and effect polymorphism); nil for fn values
	typ     *TFn        // instantiated
	effects EffSet
	ctor    *CtorInfo
	poly    []bool // param i is an effect-polymorphic function parameter
}

func (c *Checker) resolveCallee(fn syntax.Expr) *callee {
	var fi *FuncInfo
	var ci *CtorInfo
	switch f := fn.(type) {
	case *syntax.Ident:
		if c.lookupLocal(f.Name) == nil {
			if syntax.IsUpper(f.Name) {
				ci = c.lookupCtor("", f.Name, f.Span)
				if ci == nil {
					return nil
				}
			} else if fi = c.lookupFunc(f.Name); fi == nil {
				if _, ok := c.mod.Imports[f.Name]; !ok {
					c.unknownName(f.Name, f.Span)
					return nil
				}
			}
		}
	case *syntax.FieldExpr:
		if dep, alias, ok := c.moduleOf(f.X); ok {
			if dep == nil {
				return nil
			}
			if syntax.IsUpper(f.Name) {
				if ci = c.lookupCtor(alias, f.Name, f.NameSpan); ci == nil {
					return nil
				}
			} else if fi = c.memberFunc(dep, alias, f.Name, f.NameSpan); fi == nil {
				return nil
			}
		}
	}
	switch {
	case fi != nil:
		cl := &callee{name: fi.QualName(), params: fi.Params, typ: c.instantiateFn(fi), effects: fi.Effects}
		for _, p := range fi.Params {
			ft, ok := Prune(p.Type).(*TFn)
			cl.poly = append(cl.poly, ok && ft.Effects == 0)
		}
		c.ExprTypes[fn] = cl.typ
		return cl
	case ci != nil:
		t, m := c.instantiateType(ci.Type)
		ps := make([]Type, len(ci.Fields))
		var params []ParamInfo
		for i, f := range ci.Fields {
			ps[i] = Subst(f.Type, m)
			params = append(params, ParamInfo{Name: f.Name, Type: f.Type})
		}
		if len(ci.Fields) == 0 {
			c.errf("E315", fn.Sp(), "constructor `%s` takes no fields; write it without parentheses", ci.Name)
		}
		return &callee{name: ci.Name, params: params, typ: &TFn{Params: ps, Ret: t}, ctor: ci, poly: make([]bool, len(params))}
	}
	t := Prune(c.checkExpr(fn, nil))
	switch t := t.(type) {
	case *TFn:
		return &callee{name: "function value", typ: t, effects: t.Effects, poly: make([]bool, len(t.Params))}
	case *TVar:
		if t.Poison {
			return nil
		}
		c.errf("E318", fn.Sp(), "cannot infer the type of this function value").Note("annotate the parameter with a Fn(...) -> ... type")
	default:
		c.errf("E319", fn.Sp(), "a value of type %s cannot be called", Show(t))
	}
	return nil
}

type boundArg struct {
	idx  int
	expr syntax.Expr
}

func (c *Checker) checkCall(call *syntax.CallExpr, piped syntax.Expr, want Type) Type {
	cl := c.resolveCallee(call.Fn)
	args := call.Args
	if piped != nil {
		args = append([]*syntax.Arg{{Value: piped}}, args...)
	}
	if cl == nil {
		for _, a := range args {
			c.checkExpr(a.Value, nil)
		}
		return c.poison()
	}
	nparams := len(cl.typ.Params)
	var bound []boundArg
	assigned := make([]bool, nparams)
	seenNamed := false
	var needNames []*syntax.Arg
	for i, a := range args {
		if a.Name == "" {
			if seenNamed {
				c.errf("E303", a.Value.Sp(), "positional argument after a named argument")
			}
			if len(args) >= 3 && i >= 1 && cl.params != nil && i < len(cl.params) {
				needNames = append(needNames, a)
			}
			if i >= nparams {
				c.errf("E304", a.Value.Sp(), "too many arguments to %s: expected %d, found %d", cl.name, nparams, len(args))
				c.checkExpr(a.Value, nil)
				continue
			}
			assigned[i] = true
			bound = append(bound, boundArg{i, a.Value})
			continue
		}
		seenNamed = true
		if cl.params == nil {
			c.errf("E303", a.NameSpan, "named arguments cannot be used when calling a function value")
			c.checkExpr(a.Value, nil)
			continue
		}
		idx := -1
		var names []string
		for j, p := range cl.params {
			names = append(names, p.Name)
			if p.Name == a.Name {
				idx = j
			}
		}
		if idx < 0 {
			d := c.errf("E303", a.NameSpan, "%s has no parameter named `%s`", cl.name, a.Name)
			if s := diag.Suggest(a.Name, names); s != "" {
				d.WithFix("did you mean `"+s+"`?", a.NameSpan, s)
			} else {
				d.Note("parameters are: %s", strings.Join(names, ", "))
			}
			c.checkExpr(a.Value, nil)
			continue
		}
		if assigned[idx] {
			c.errf("E303", a.NameSpan, "argument `%s` is given twice", a.Name)
			continue
		}
		assigned[idx] = true
		bound = append(bound, boundArg{idx, a.Value})
	}
	if len(needNames) > 0 {
		d := c.errf("E303", needNames[0].Value.Sp(), "calls with 3 or more arguments must name every argument after the first")
		var edits []diag.Edit
		for _, a := range needNames {
			for j, b := range bound {
				if b.expr == a.Value {
					sp := a.Value.Sp()
					edits = append(edits, diag.Edit{Span: diag.Span{File: sp.File, Start: sp.Start, End: sp.Start}, Text: cl.params[bound[j].idx].Name + ": "})
				}
			}
		}
		d.Fixes = append(d.Fixes, diag.Fix{Message: "name the arguments", Edits: edits})
		d.Note("this makes argument order mistakes impossible")
	}
	var missing []string
	for i, ok := range assigned {
		if !ok {
			if cl.params != nil {
				missing = append(missing, "`"+cl.params[i].Name+"`")
			} else {
				missing = append(missing, "#"+itoa(i+1))
			}
		}
	}
	if len(missing) > 0 {
		c.errf("E304", call.Span, "missing argument %s in call to %s", fmtList(missing), cl.name)
	}
	eff := cl.effects
	for _, b := range bound {
		pt := cl.typ.Params[b.idx]
		at := c.checkExpr(b.expr, pt)
		what := "argument"
		if cl.params != nil {
			what = "argument `" + cl.params[b.idx].Name + "` of " + cl.name
		}
		c.unifyAt(pt, at, b.expr.Sp(), what)
		if af, ok := Prune(at).(*TFn); ok {
			if b.idx < len(cl.poly) && cl.poly[b.idx] {
				eff |= af.Effects
			} else if pf, ok := Prune(pt).(*TFn); ok && af.Effects&^pf.Effects != 0 {
				c.errf("E402", b.expr.Sp(), "this function uses effect `%s`, which parameter `%s` does not allow", (af.Effects &^ pf.Effects).String(), paramName(cl, b.idx))
			}
		}
	}
	c.require(eff, call.Span, "calling "+cl.name)
	c.checkSpecialConstraints(cl, call)
	return cl.typ.Ret
}

func paramName(cl *callee, i int) string {
	if cl.params != nil && i < len(cl.params) {
		return cl.params[i].Name
	}
	return "#" + itoa(i+1)
}

// checkSpecialConstraints enforces constraints the type system cannot yet
// express (ordering on sort keys).
func (c *Checker) checkSpecialConstraints(cl *callee, call *syntax.CallExpr) {
	var t Type
	switch cl.name {
	case "list.sort", "list.sort_desc", "list.max", "list.min":
		if l, ok := Prune(cl.typ.Params[0]).(*TCon); ok && len(l.Args) == 1 {
			t = l.Args[0]
		}
	case "list.sort_by", "list.sort_by_desc", "list.min_by", "list.max_by":
		if f, ok := Prune(cl.typ.Params[1]).(*TFn); ok {
			t = f.Ret
		}
	default:
		return
	}
	if t != nil && !isUnresolved(t) && !isCon(t, "Int") && !isCon(t, "Float") && !isCon(t, "Str") {
		c.errf("E320", call.Span, "%s needs Int, Float or Str values to compare, found %s", cl.name, Show(t)).
			Note("use list.sort_by(xs, fn(x) => x.some_key)")
	}
}

func itoa(i int) string { return strconv.Itoa(i) }

// ---- operators ----

func (c *Checker) checkBinary(e *syntax.BinaryExpr) Type {
	switch e.Op {
	case syntax.KW_AND, syntax.KW_OR:
		c.expectType(e.L, TBool, "`"+e.Op.String()+"` operand")
		c.expectType(e.R, TBool, "`"+e.Op.String()+"` operand")
		return TBool
	}
	lt := c.checkExpr(e.L, nil)
	rt := c.checkExpr(e.R, lt)
	if isPoison(lt) || isPoison(rt) {
		Unify(lt, rt)
		if isCmp(e.Op) {
			return TBool
		}
		return c.poison()
	}
	if !Unify(lt, rt) {
		d := c.errf("E310", e.OpSpan, "operator `%s` needs both sides to have the same type, found %s and %s", e.Op, Show(lt), Show(rt))
		if (isCon(lt, "Int") && isCon(rt, "Float")) || (isCon(lt, "Float") && isCon(rt, "Int")) {
			d.Note("convert explicitly with math.to_float(x)")
		}
		if e.Op == syntax.PLUS && (isCon(lt, "Str") || isCon(rt, "Str")) {
			d.Note("build strings with interpolation: \"${a}${b}\"")
		}
		if isCmp(e.Op) {
			return TBool
		}
		return c.poison()
	}
	t := Prune(lt)
	if _, ok := t.(*TVar); ok {
		c.errf("E318", e.Span, "cannot infer the operand type of `%s`", e.Op).Note("annotate the lambda parameters or bindings involved")
		if isCmp(e.Op) {
			return TBool
		}
		return t
	}
	switch e.Op {
	case syntax.EQ, syntax.NE:
		if containsFn(t) {
			c.errf("E310", e.OpSpan, "functions cannot be compared with `%s`", e.Op)
		}
		return TBool
	case syntax.LT, syntax.LE, syntax.GT, syntax.GE:
		if !isCon(t, "Int") && !isCon(t, "Float") && !isCon(t, "Str") {
			c.errf("E310", e.OpSpan, "operator `%s` needs Int, Float or Str operands, found %s", e.Op, Show(t))
		}
		return TBool
	case syntax.PLUS:
		if !isCon(t, "Int") && !isCon(t, "Float") && !isCon(t, "Str") && !isCon(t, "List") {
			c.errf("E310", e.OpSpan, "operator `+` needs Int, Float, Str or List operands, found %s", Show(t))
		}
		return t
	default:
		if !isCon(t, "Int") && !isCon(t, "Float") {
			c.errf("E310", e.OpSpan, "operator `%s` needs Int or Float operands, found %s", e.Op, Show(t))
		}
		return t
	}
}

func isCmp(k syntax.Kind) bool {
	switch k {
	case syntax.EQ, syntax.NE, syntax.LT, syntax.LE, syntax.GT, syntax.GE:
		return true
	}
	return false
}

// ---- records ----

func (c *Checker) checkRecordLit(e *syntax.RecordLit) Type {
	var ti *TypeInfo
	if e.Module != "" {
		dep, ok := c.mod.Imports[e.Module]
		if !ok {
			c.unknownModule(e.Module, e.Span)
		} else if ti = dep.Types[e.Name]; ti == nil {
			c.errf("E203", e.Span, "module `%s` has no record `%s`", e.Module, e.Name)
		} else if !ti.Pub {
			c.errf("E206", e.Span, "record `%s.%s` is not `pub`", e.Module, e.Name)
		}
	} else {
		ti = c.mod.Types[e.Name]
		if ti == nil && c.prelude != nil {
			ti = c.prelude.Types[e.Name]
		}
		if ti == nil {
			d := c.errf("E203", e.Span, "unknown record type `%s`", e.Name)
			if s := diag.Suggest(e.Name, keys(c.mod.Types)); s != "" {
				d.Note("did you mean `%s`?", s)
			}
		}
	}
	if ti == nil || !ti.IsRecord {
		if ti != nil {
			c.errf("E321", e.Span, "`%s` is a sum type, not a record; construct a variant like `%s(...)`", e.Name, firstCtor(ti))
		}
		if e.Base != nil {
			c.checkExpr(e.Base, nil)
		}
		for _, f := range e.Fields {
			c.checkExpr(f.Value, nil)
		}
		return c.fresh()
	}
	t, m := c.instantiateType(ti)
	if e.Base != nil {
		c.unifyAt(t, c.checkExpr(e.Base, t), e.Base.Sp(), "record update base")
	}
	given := map[string]bool{}
	for _, f := range e.Fields {
		fd, _, ok := ti.Field(f.Name)
		if !ok {
			d := c.errf("E317", f.NameSpan, "record `%s` has no field `%s`", ti.Name, f.Name)
			var names []string
			for _, x := range ti.Fields {
				names = append(names, x.Name)
			}
			if s := diag.Suggest(f.Name, names); s != "" {
				d.WithFix("did you mean `"+s+"`?", f.NameSpan, s)
			}
			c.checkExpr(f.Value, nil)
			continue
		}
		if given[f.Name] {
			c.errf("E317", f.NameSpan, "field `%s` is given twice", f.Name)
		}
		given[f.Name] = true
		ft := Subst(fd.Type, m)
		c.unifyAt(ft, c.checkExpr(f.Value, ft), f.Value.Sp(), "field `"+f.Name+"`")
	}
	if e.Base == nil {
		var missing []string
		for _, f := range ti.Fields {
			if !given[f.Name] {
				missing = append(missing, f.Name)
			}
		}
		if len(missing) > 0 {
			d := c.errf("E322", e.Span, "missing field(s) %s in `%s` literal", fmtList(quote(missing)), ti.Name)
			var add []string
			for _, n := range missing {
				add = append(add, n+": ???")
			}
			end := e.Span.End
			end.Col--
			end.Offset--
			prefix := ", "
			if len(e.Fields) == 0 {
				prefix = ""
			}
			d.WithFix("add the missing fields", diag.Span{File: e.Span.File, Start: end, End: end}, prefix+strings.Join(add, ", "))
		}
	}
	return t
}

func quote(xs []string) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = "`" + x + "`"
	}
	return out
}

func firstCtor(ti *TypeInfo) string {
	if len(ti.Ctors) > 0 {
		return ti.Ctors[0].Name
	}
	return ti.Name
}

// ---- lambdas ----

func (c *Checker) checkLambda(e *syntax.LambdaExpr, want Type) Type {
	wf, _ := Prune(want).(*TFn)
	if wf != nil && len(wf.Params) != len(e.Params) {
		c.errf("E323", e.Span, "this function takes %d parameter(s), but %d are expected here", len(e.Params), len(wf.Params))
		wf = nil
	}
	ps := make([]Type, len(e.Params))
	for i, p := range e.Params {
		switch {
		case p.Type != nil:
			ps[i] = c.resolveType(p.Type, c.genericEnv())
			if wf != nil {
				c.unifyAt(wf.Params[i], ps[i], p.Span, "lambda parameter `"+p.Name+"`")
			}
		case wf != nil:
			ps[i] = wf.Params[i]
		default:
			ps[i] = c.fresh()
		}
	}
	var ret Type
	switch {
	case e.Ret != nil:
		ret = c.resolveType(e.Ret, c.genericEnv())
	case wf != nil:
		ret = wf.Ret
	default:
		ret = c.fresh()
	}
	var eff EffSet
	saved := c.fn
	c.fn = &fnCtx{name: "lambda", ret: ret, effects: 0, collect: &eff, root: saved.root}
	c.pushScope()
	for i, p := range e.Params {
		c.declare(p.Name, ps[i], false, p.Span, diag.Span{}, true)
	}
	if isCon(ret, "Unit") {
		c.checkBlock(e.Body, nil)
	} else {
		c.checkBlock(e.Body, ret)
	}
	c.popScope()
	c.fn = saved
	return &TFn{Params: ps, Ret: ret, Effects: eff}
}

// ---- if / match ----

func (c *Checker) checkIf(e *syntax.IfExpr, want Type) Type {
	for _, br := range e.Branches {
		c.expectType(br.Cond, TBool, "`if` condition")
	}
	if want == nil {
		for _, br := range e.Branches {
			c.checkBlock(br.Body, nil)
		}
		if e.Else != nil {
			c.checkBlock(e.Else, nil)
		}
		return TUnit
	}
	if e.Else == nil && !isCon(want, "Unit") {
		if !Unify(want, TUnit) {
			c.errf("E324", e.Span, "`if` without `else` cannot produce a value of type %s", Show(want)).
				Note("add an `else` branch")
		}
	}
	for _, br := range e.Branches {
		c.checkBlock(br.Body, want)
	}
	if e.Else != nil {
		c.checkBlock(e.Else, want)
	}
	return want
}

func (c *Checker) checkTry(e *syntax.TryExpr) Type {
	t := c.checkExpr(e.X, nil)
	ret := c.fn.ret
	if c.fn.root.isTest && c.fn.collect == nil {
		c.errf("E325", e.Span, "`?` cannot be used in a test").Note("match on the value, or `expect x == Ok(...)`")
		return c.fresh()
	}
	switch {
	case isRes(t):
		pt := Prune(t).(*TCon)
		errT := pt.Args[1]
		want, _ := c.instantiateType(c.prelude.Types["Result"])
		if !Unify(ret, want) {
			c.errf("E325", e.Span, "`?` on a Result needs the enclosing function to return Result[_, %s], but it returns %s", Show(errT), Show(ret)).
				Note("change the return type, or handle the error with `match`")
			return pt.Args[0]
		}
		if !Unify(want.Args[1], errT) {
			c.errf("E325", e.Span, "`?` propagates an error of type %s, but the function returns errors of type %s", Show(errT), Show(want.Args[1])).
				Note("convert the error first: result.map_err(x, fn(e) => ...)")
		}
		return pt.Args[0]
	case isOpt(t):
		pt := Prune(t).(*TCon)
		want, _ := c.instantiateType(c.prelude.Types["Option"])
		if !Unify(ret, want) {
			c.errf("E325", e.Span, "`?` on an Option needs the enclosing function to return an Option, but it returns %s", Show(ret)).
				Note("convert it: option.ok_or(x, \"error message\")?")
		}
		return pt.Args[0]
	case isUnresolved(t):
		c.errf("E318", e.Span, "cannot infer the type of the value before `?`")
		return c.fresh()
	}
	c.errf("E325", e.Span, "`?` works on Result or Option values, found %s", Show(t))
	return c.fresh()
}
