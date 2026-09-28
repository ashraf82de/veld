package interp

import (
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/ashraf82de/veld/internal/diag"
	"github.com/ashraf82de/veld/internal/syntax"
	"github.com/ashraf82de/veld/internal/types"
)

// Interp runs a checked program.
type Interp struct {
	Prog    *types.Program
	Granted types.EffSet
	Sources map[string]string
	Stdout  func(string)
	Stderr  func(string)
	Args    []string
	natives map[string]*Native
	outMu   sync.Mutex
}

// Env is a lexical scope.
type Env struct {
	vars   map[string]Value
	parent *Env
}

func NewEnv(parent *Env) *Env { return &Env{vars: make(map[string]Value, 4), parent: parent} }

func (e *Env) Lookup(name string) (Value, bool) {
	for s := e; s != nil; s = s.parent {
		if v, ok := s.vars[name]; ok {
			return v, true
		}
	}
	return nil, false
}

func (e *Env) Set(name string, v Value) {
	for s := e; s != nil; s = s.parent {
		if _, ok := s.vars[name]; ok {
			s.vars[name] = v
			return
		}
	}
	e.vars[name] = v
}

func (e *Env) Define(name string, v Value) {
	if name != "_" {
		e.vars[name] = v
	}
}

// Frame is one entry of the Veld call stack.
type Frame struct {
	Function string    `json:"function"`
	Span     diag.Span `json:"span"`
}

// Thread is the state of one evaluation (the main program, one test, or one
// HTTP request).
type Thread struct {
	in    *Interp
	stack []Frame
	mod   *types.Module
}

// RuntimeError is a Veld runtime failure with a Veld stack trace.
type RuntimeError struct {
	Code    string    `json:"code"`
	Message string    `json:"message"`
	Span    diag.Span `json:"span"`
	Stack   []Frame   `json:"stack"`
	Details []string  `json:"details,omitempty"`
}

func (e *RuntimeError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: runtime error[%s]: %s", e.Span, e.Code, e.Message)
	for _, d := range e.Details {
		fmt.Fprintf(&b, "\n  %s", d)
	}
	for i := len(e.Stack) - 1; i >= 0; {
		f := e.Stack[i]
		n := 1
		for i-n >= 0 && e.Stack[i-n] == f {
			n++
		}
		fmt.Fprintf(&b, "\n  in %s, called at %s", f.Function, f.Span)
		if n > 1 {
			fmt.Fprintf(&b, " (x%d)", n)
		}
		i -= n
	}
	return b.String()
}

// ExitRequest is raised by env.exit.
type ExitRequest struct{ Code int }

type returnSignal struct{ v Value }
type breakSignal struct{}
type continueSignal struct{}

const maxDepth = 10000

func New(prog *types.Program, granted types.EffSet, sources map[string]string) *Interp {
	in := &Interp{Prog: prog, Granted: granted, Sources: sources,
		Stdout: func(s string) { fmt.Print(s) }, Stderr: func(s string) { fmt.Print(s) }}
	in.natives = natives()
	return in
}

func (in *Interp) NewThread(mod *types.Module) *Thread {
	return &Thread{in: in, mod: mod}
}

func (th *Thread) fail(code string, sp diag.Span, format string, args ...any) {
	panic(&RuntimeError{Code: code, Message: fmt.Sprintf(format, args...), Span: sp, Stack: append([]Frame{}, th.stack...)})
}

// Protect runs f and converts runtime panics into errors.
func (th *Thread) Protect(f func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			switch r := r.(type) {
			case *RuntimeError:
				err = r
			case *ExitRequest:
				panic(r)
			case returnSignal, breakSignal, continueSignal:
				err = &RuntimeError{Code: "R999", Message: "control flow escaped its function"}
			default:
				if e, ok := r.(error); ok && strings.Contains(e.Error(), "stack overflow") {
					err = &RuntimeError{Code: "R501", Message: "stack overflow"}
					return
				}
				panic(r)
			}
		}
	}()
	f()
	return nil
}

// CallMain runs the entry function `main`.
func (th *Thread) CallFunc(fi *types.FuncInfo, args []Value) Value {
	return th.callFunc(fi, args, fi.Decl.NameSpan)
}

// ---- functions ----

func (th *Thread) callValue(fv Value, args []Value, sp diag.Span) Value {
	switch f := fv.(type) {
	case *FuncRef:
		return th.callFunc(f.Info, args, sp)
	case *Closure:
		return th.callClosure(f, args, sp)
	case *Native:
		return f.Fn(th, args)
	}
	th.fail("R999", sp, "value is not callable")
	return nil
}

func (th *Thread) push(name string, sp diag.Span) {
	if len(th.stack) >= maxDepth {
		th.fail("R501", sp, "stack overflow: more than %d nested calls", maxDepth)
	}
	th.stack = append(th.stack, Frame{Function: name, Span: sp})
}

func (th *Thread) callFunc(fi *types.FuncInfo, args []Value, sp diag.Span) (result Value) {
	if fi.Extern {
		if fi.Effects&^th.in.Granted != 0 {
			th.fail("R401", sp, "effect `%s` was not granted to this program", (fi.Effects &^ th.in.Granted).String())
		}
		n := th.in.natives[fi.QualName()]
		if n == nil {
			th.fail("R999", sp, "native function %s is not implemented", fi.QualName())
		}
		th.push(fi.QualName(), sp)
		v := n.Fn(th, args)
		th.stack = th.stack[:len(th.stack)-1]
		return v
	}
	d := fi.Decl
	th.push(fi.QualName(), sp)
	savedMod := th.mod
	th.mod = fi.Module
	env := NewEnv(nil)
	for i, p := range d.Params {
		env.Define(p.Name, args[i])
	}
	for _, r := range d.Requires {
		if v := th.eval(r, env); v != true {
			th.contractFail("requires", r, env)
		}
	}
	result = th.runBody(d.Body, env)
	if len(d.Ensures) > 0 {
		env2 := NewEnv(env)
		env2.Define("result", result)
		for _, e := range d.Ensures {
			if v := th.eval(e, env2); v != true {
				th.contractFail("ensures", e, env2)
			}
		}
	}
	th.mod = savedMod
	th.stack = th.stack[:len(th.stack)-1]
	return result
}

func (th *Thread) contractFail(kind string, e syntax.Expr, env *Env) {
	err := &RuntimeError{Code: "R201", Message: fmt.Sprintf("contract violated: %s %s", kind, th.src(e)), Span: e.Sp(), Stack: append([]Frame{}, th.stack...)}
	err.Details = th.explain(e, env)
	panic(err)
}

// explain shows the operand values of a failed boolean comparison.
func (th *Thread) explain(e syntax.Expr, env *Env) []string {
	if b, ok := e.(*syntax.BinaryExpr); ok && isCmpOp(b.Op) {
		return []string{"left:  " + Repr(th.eval(b.L, env)), "right: " + Repr(th.eval(b.R, env))}
	}
	return nil
}

func isCmpOp(k syntax.Kind) bool {
	switch k {
	case syntax.EQ, syntax.NE, syntax.LT, syntax.LE, syntax.GT, syntax.GE:
		return true
	}
	return false
}

func (th *Thread) src(n syntax.Node) string {
	sp := n.Sp()
	s, ok := th.in.Sources[sp.File]
	if !ok || sp.End.Offset > len(s) || sp.Start.Offset > sp.End.Offset {
		return ""
	}
	return s[sp.Start.Offset:sp.End.Offset]
}

func (th *Thread) runBody(b *syntax.Block, env *Env) (result Value) {
	defer func() {
		if r := recover(); r != nil {
			if rs, ok := r.(returnSignal); ok {
				result = rs.v
				return
			}
			panic(r)
		}
	}()
	return th.execBlockIn(b, env)
}

func (th *Thread) callClosure(c *Closure, args []Value, sp diag.Span) Value {
	th.push("lambda", sp)
	savedMod := th.mod
	th.mod = c.Mod
	env := NewEnv(c.Env)
	for i, p := range c.Params {
		env.Define(p, args[i])
	}
	v := th.runBody(c.Body, env)
	th.mod = savedMod
	th.stack = th.stack[:len(th.stack)-1]
	return v
}

// ---- blocks ----

func (th *Thread) execBlock(b *syntax.Block, env *Env) Value {
	return th.execBlockIn(b, NewEnv(env))
}

func (th *Thread) execBlockIn(b *syntax.Block, env *Env) Value {
	var last Value = Unit
	for _, s := range b.Stmts {
		last = th.exec(s, env)
	}
	return last
}

func (th *Thread) exec(s syntax.Stmt, env *Env) Value {
	switch s := s.(type) {
	case *syntax.LetStmt:
		env.Define(s.Name, th.eval(s.Value, env))
		return Unit
	case *syntax.SetStmt:
		env.Set(s.Name, th.eval(s.Value, env))
		return Unit
	case *syntax.ExprStmt:
		return th.eval(s.X, env)
	case *syntax.ExpectStmt:
		if v := th.eval(s.X, env); v != true {
			err := &RuntimeError{Code: "T001", Message: "expectation failed: expect " + th.src(s.X), Span: s.X.Sp(), Stack: append([]Frame{}, th.stack...)}
			err.Details = th.explain(s.X, env)
			panic(err)
		}
		return Unit
	}
	return Unit
}

// ---- expressions ----

func (th *Thread) eval(e syntax.Expr, env *Env) Value {
	switch e := e.(type) {
	case *syntax.IntLit:
		return e.Value
	case *syntax.FloatLit:
		return e.Value
	case *syntax.BoolLit:
		return e.Value
	case *syntax.StrLit:
		if len(e.Segs) == 1 && e.Segs[0].Expr == nil {
			return e.Segs[0].Lit
		}
		var b strings.Builder
		for _, s := range e.Segs {
			if s.Expr != nil {
				b.WriteString(Show(th.eval(s.Expr, env)))
			} else {
				b.WriteString(s.Lit)
			}
		}
		return b.String()
	case *syntax.ParenExpr:
		return th.eval(e.X, env)
	case *syntax.Ident:
		return th.lookup(e, env)
	case *syntax.FieldExpr:
		if mod, ok := th.moduleOf(e.X, env); ok {
			return th.member(mod, e.Name, e.Span)
		}
		x := th.eval(e.X, env)
		r := x.(*Record)
		_, i, _ := r.Type.Field(e.Name)
		return r.Fields[i]
	case *syntax.CallExpr:
		return th.evalCall(e, nil, env)
	case *syntax.PipeExpr:
		return th.evalCall(e.R, e.L, env)
	case *syntax.BinaryExpr:
		return th.evalBinary(e, env)
	case *syntax.UnaryExpr:
		v := th.eval(e.X, env)
		if e.Op == syntax.KW_NOT {
			return !v.(bool)
		}
		switch v := v.(type) {
		case int64:
			if v == math.MinInt64 {
				th.fail("R101", e.Span, "integer overflow in negation")
			}
			return -v
		case float64:
			return -v
		}
	case *syntax.ListLit:
		out := make([]Value, len(e.Elems))
		for i, x := range e.Elems {
			out[i] = th.eval(x, env)
		}
		return NewList(out)
	case *syntax.MapLit:
		m := NewMap()
		for _, en := range e.Entries {
			m = m.Put(th.eval(en.Key, env), th.eval(en.Value, env))
		}
		return m
	case *syntax.RecordLit:
		ti := th.recordType(e)
		r := &Record{Type: ti, Fields: make([]Value, len(ti.Fields))}
		if e.Base != nil {
			base := th.eval(e.Base, env).(*Record)
			copy(r.Fields, base.Fields)
		}
		for _, f := range e.Fields {
			_, i, _ := ti.Field(f.Name)
			r.Fields[i] = th.eval(f.Value, env)
		}
		return r
	case *syntax.LambdaExpr:
		ps := make([]string, len(e.Params))
		for i, p := range e.Params {
			ps[i] = p.Name
		}
		return &Closure{Params: ps, Body: e.Body, Env: env, Mod: th.mod}
	case *syntax.IfExpr:
		for _, br := range e.Branches {
			if th.eval(br.Cond, env) == true {
				return th.execBlock(br.Body, env)
			}
		}
		if e.Else != nil {
			return th.execBlock(e.Else, env)
		}
		return Unit
	case *syntax.MatchExpr:
		return th.evalMatch(e, env)
	case *syntax.ForExpr:
		items := th.eval(e.Iter, env).(List).ToSlice()
		for _, it := range items {
			if th.loopBody(e.Body, env, e.Var, it) {
				break
			}
		}
		return Unit
	case *syntax.WhileExpr:
		for th.eval(e.Cond, env) == true {
			if th.loopBody(e.Body, env, "", nil) {
				break
			}
		}
		return Unit
	case *syntax.ReturnExpr:
		var v Value = Unit
		if e.X != nil {
			v = th.eval(e.X, env)
		}
		panic(returnSignal{v})
	case *syntax.BreakExpr:
		panic(breakSignal{})
	case *syntax.ContinueExpr:
		panic(continueSignal{})
	case *syntax.TodoExpr:
		th.fail("R301", e.Span, "reached a `???` hole")
	case *syntax.TryExpr:
		v := th.eval(e.X, env).(*Variant)
		switch v.Ctor.Name {
		case "Ok", "Some":
			return v.Fields[0]
		default:
			panic(returnSignal{v})
		}
	}
	th.fail("R999", e.Sp(), "cannot evaluate expression")
	return nil
}

// loopBody runs one iteration; it reports whether the loop should stop.
func (th *Thread) loopBody(b *syntax.Block, env *Env, name string, v Value) (stop bool) {
	defer func() {
		if r := recover(); r != nil {
			switch r.(type) {
			case breakSignal:
				stop = true
			case continueSignal:
				stop = false
			default:
				panic(r)
			}
		}
	}()
	inner := NewEnv(env)
	if name != "" {
		inner.Define(name, v)
	}
	th.execBlockIn(b, inner)
	return false
}

func (th *Thread) lookup(e *syntax.Ident, env *Env) Value {
	if v, ok := env.Lookup(e.Name); ok {
		return v
	}
	mod := th.mod
	if e.Name == "Unit" {
		return Unit
	}
	if syntax.IsUpper(e.Name) {
		ci := mod.Ctors[e.Name]
		if ci == nil {
			ci = th.in.Prog.Prelude.Ctors[e.Name]
		}
		return &Variant{Ctor: ci}
	}
	if fi := mod.Funcs[e.Name]; fi != nil {
		return &FuncRef{Info: fi}
	}
	if fi := th.in.Prog.Prelude.Funcs[e.Name]; fi != nil {
		return &FuncRef{Info: fi}
	}
	th.fail("R999", e.Span, "unknown name %s", e.Name)
	return nil
}

func (th *Thread) moduleOf(x syntax.Expr, env *Env) (*types.Module, bool) {
	id, ok := x.(*syntax.Ident)
	if !ok {
		return nil, false
	}
	if _, isLocal := env.Lookup(id.Name); isLocal {
		return nil, false
	}
	m, ok := th.mod.Imports[id.Name]
	return m, ok
}

func (th *Thread) member(m *types.Module, name string, sp diag.Span) Value {
	if syntax.IsUpper(name) {
		return &Variant{Ctor: m.Ctors[name]}
	}
	fi := m.Funcs[name]
	if fi == nil {
		th.fail("R999", sp, "unknown member %s", name)
	}
	return &FuncRef{Info: fi}
}

func (th *Thread) recordType(e *syntax.RecordLit) *types.TypeInfo {
	if e.Module != "" {
		return th.mod.Imports[e.Module].Types[e.Name]
	}
	if ti := th.mod.Types[e.Name]; ti != nil {
		return ti
	}
	return th.in.Prog.Prelude.Types[e.Name]
}

// ctorOf returns the constructor a call targets, if any.
func (th *Thread) ctorOf(fn syntax.Expr, env *Env) *types.CtorInfo {
	switch f := fn.(type) {
	case *syntax.Ident:
		if syntax.IsUpper(f.Name) {
			if ci := th.mod.Ctors[f.Name]; ci != nil {
				return ci
			}
			return th.in.Prog.Prelude.Ctors[f.Name]
		}
	case *syntax.FieldExpr:
		if syntax.IsUpper(f.Name) {
			if m, ok := th.moduleOf(f.X, env); ok {
				return m.Ctors[f.Name]
			}
		}
	}
	return nil
}

func (th *Thread) evalCall(c *syntax.CallExpr, piped syntax.Expr, env *Env) Value {
	args := c.Args
	if piped != nil {
		args = append([]*syntax.Arg{{Value: piped}}, args...)
	}
	if ci := th.ctorOf(c.Fn, env); ci != nil {
		vals := th.bindArgs(args, fieldNames(ci.Fields), env)
		return &Variant{Ctor: ci, Fields: vals}
	}
	fv := th.eval(c.Fn, env)
	var names []string
	if f, ok := fv.(*FuncRef); ok {
		for _, p := range f.Info.Params {
			names = append(names, p.Name)
		}
	}
	vals := th.bindArgs(args, names, env)
	return th.callValue(fv, vals, c.Span)
}

func fieldNames(fs []types.FieldInfo) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Name
	}
	return out
}

// bindArgs evaluates arguments in source order and places them by name.
func (th *Thread) bindArgs(args []*syntax.Arg, names []string, env *Env) []Value {
	n := len(args)
	if len(names) > n {
		n = len(names)
	}
	out := make([]Value, n)
	for i, a := range args {
		v := th.eval(a.Value, env)
		if a.Name == "" {
			out[i] = v
			continue
		}
		for j, nm := range names {
			if nm == a.Name {
				out[j] = v
				break
			}
		}
	}
	return out[:max(len(names), len(args))]
}

// ---- operators ----

func (th *Thread) evalBinary(e *syntax.BinaryExpr, env *Env) Value {
	switch e.Op {
	case syntax.KW_AND:
		if th.eval(e.L, env) != true {
			return false
		}
		return th.eval(e.R, env) == true
	case syntax.KW_OR:
		if th.eval(e.L, env) == true {
			return true
		}
		return th.eval(e.R, env) == true
	}
	l := th.eval(e.L, env)
	r := th.eval(e.R, env)
	switch e.Op {
	case syntax.EQ:
		return Equal(l, r)
	case syntax.NE:
		return !Equal(l, r)
	case syntax.LT:
		return Compare(l, r) < 0
	case syntax.LE:
		return Compare(l, r) <= 0
	case syntax.GT:
		return Compare(l, r) > 0
	case syntax.GE:
		return Compare(l, r) >= 0
	}
	switch a := l.(type) {
	case int64:
		b := r.(int64)
		switch e.Op {
		case syntax.PLUS:
			if (b > 0 && a > math.MaxInt64-b) || (b < 0 && a < math.MinInt64-b) {
				th.fail("R101", e.OpSpan, "integer overflow: %d + %d", a, b)
			}
			return a + b
		case syntax.MINUS:
			if (b < 0 && a > math.MaxInt64+b) || (b > 0 && a < math.MinInt64+b) {
				th.fail("R101", e.OpSpan, "integer overflow: %d - %d", a, b)
			}
			return a - b
		case syntax.STAR:
			if a != 0 && b != 0 {
				p := a * b
				if p/b != a || (a == -1 && b == math.MinInt64) || (b == -1 && a == math.MinInt64) {
					th.fail("R101", e.OpSpan, "integer overflow: %d * %d", a, b)
				}
				return p
			}
			return int64(0)
		case syntax.SLASH, syntax.PERCENT:
			if b == 0 {
				th.fail("R102", e.OpSpan, "division by zero")
			}
			if a == math.MinInt64 && b == -1 {
				th.fail("R101", e.OpSpan, "integer overflow in division")
			}
			if e.Op == syntax.SLASH {
				return a / b
			}
			return a % b
		}
	case float64:
		b := r.(float64)
		switch e.Op {
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
	case string:
		return a + r.(string)
	case List:
		return a.Concat(r.(List))
	}
	th.fail("R999", e.Span, "bad operands for %s", e.Op)
	return nil
}

// ---- match ----

func (th *Thread) evalMatch(e *syntax.MatchExpr, env *Env) Value {
	v := th.eval(e.X, env)
	for _, arm := range e.Arms {
		armEnv := NewEnv(env)
		if !th.bind(arm.Pattern, v, armEnv) {
			continue
		}
		if arm.Guard != nil && th.eval(arm.Guard, armEnv) != true {
			continue
		}
		return th.execBlockIn(arm.Body, armEnv)
	}
	th.fail("R303", e.Span, "no case matched value %s", Repr(v))
	return nil
}

func (th *Thread) bind(p syntax.Pattern, v Value, env *Env) bool {
	switch p := p.(type) {
	case *syntax.WildPat:
		return true
	case *syntax.OrPat:
		for _, a := range p.Alts {
			if th.bind(a, v, env) {
				return true
			}
		}
		return false
	case *syntax.BindPat:
		env.Define(p.Name, v)
		return true
	case *syntax.LitPat:
		switch lit := p.Value.(type) {
		case *syntax.IntLit:
			return v == lit.Value
		case *syntax.BoolLit:
			return v == lit.Value
		case *syntax.StrLit:
			s := ""
			for _, seg := range lit.Segs {
				s += seg.Lit
			}
			return v == s
		}
		return false
	case *syntax.CtorPat:
		vv, ok := v.(*Variant)
		if !ok || vv.Ctor.Name != p.Name {
			return false
		}
		for i, a := range p.Args {
			if !th.bind(a, vv.Fields[i], env) {
				return false
			}
		}
		return true
	case *syntax.ListPat:
		l := v.(List)
		n := l.Len()
		fixed := len(p.Elems) + len(p.Suffix)
		if p.HasRest {
			if n < fixed {
				return false
			}
		} else if n != fixed {
			return false
		}
		for i, ep := range p.Elems {
			if !th.bind(ep, l.Get(i), env) {
				return false
			}
		}
		off := n - len(p.Suffix)
		for i, ep := range p.Suffix {
			if !th.bind(ep, l.Get(off+i), env) {
				return false
			}
		}
		if p.HasRest && p.Rest != "" && p.Rest != "_" {
			env.Define(p.Rest, l.Slice(len(p.Elems), off))
		}
		return true
	}
	return false
}

// ---- tests ----

// TestResult is the outcome of one test block.
type TestResult struct {
	Name   string        `json:"name"`
	Module string        `json:"module"`
	Span   diag.Span     `json:"span"`
	Passed bool          `json:"passed"`
	Error  *RuntimeError `json:"error,omitempty"`
}

// RunTests runs every test in mod.
func (in *Interp) RunTests(mod *types.Module, filter string) []TestResult {
	var out []TestResult
	for _, t := range mod.Tests {
		if filter != "" && !strings.Contains(t.Name, filter) {
			continue
		}
		th := in.NewThread(mod)
		th.stack = nil
		err := th.Protect(func() { th.execBlock(t.Body, NewEnv(nil)) })
		r := TestResult{Name: t.Name, Module: mod.Path, Span: t.Span, Passed: err == nil}
		if err != nil {
			r.Error = err.(*RuntimeError)
		}
		out = append(out, r)
	}
	return out
}
