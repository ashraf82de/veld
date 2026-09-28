package interp

import (
	"fmt"
	"strings"
	"sync"

	"github.com/ashraf82de/veld/internal/diag"
	"github.com/ashraf82de/veld/internal/syntax"
	"github.com/ashraf82de/veld/internal/types"
)

// Interp runs a checked program. Functions are compiled once, when the
// interpreter is created, into trees of Go closures (see compile*.go); the
// interpreter itself only holds shared state and can serve many threads.
type Interp struct {
	Prog    *types.Program
	Granted types.EffSet
	Sources map[string]string
	Stdout  func(string)
	Stderr  func(string)
	Args    []string
	natives map[string]*Native
	outMu   sync.Mutex
	fns     map[*types.FuncInfo]*fnCode
}

// Frame is one entry of the Veld call stack.
type Frame struct {
	Function string    `json:"function"`
	Span     diag.Span `json:"span"`
}

type callSite struct {
	name string
	span diag.Span
}

type stackEntry struct {
	name string
	site *callSite
}

var noSite = &callSite{}

// Thread is the state of one evaluation (the main program, one test, or one
// HTTP request). Threads are not safe for concurrent use.
type Thread struct {
	in    *Interp
	stack []stackEntry
	mod   *types.Module
	free  []*frame
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

// maxDepth bounds Veld call nesting; deeper recursion is reported as a
// runtime error (R501) instead of exhausting the Go stack.
const maxDepth = 100_000

// New compiles prog and returns an interpreter for it.
func New(prog *types.Program, granted types.EffSet, sources map[string]string) *Interp {
	in := &Interp{Prog: prog, Granted: granted, Sources: sources,
		Stdout: func(s string) { fmt.Print(s) }, Stderr: func(s string) { fmt.Print(s) }}
	in.natives = natives()
	in.compileProgram()
	return in
}

// NewThread returns a fresh thread whose stack starts empty.
func (in *Interp) NewThread(mod *types.Module) *Thread {
	return &Thread{in: in, mod: mod}
}

func (th *Thread) trace() []Frame {
	out := make([]Frame, len(th.stack))
	for i, e := range th.stack {
		out[i] = Frame{Function: e.name, Span: e.site.span}
	}
	return out
}

func (th *Thread) fail(code string, sp diag.Span, format string, args ...any) {
	panic(&RuntimeError{Code: code, Message: fmt.Sprintf(format, args...), Span: sp, Stack: th.trace()})
}

// site is the call site of the innermost active call.
func (th *Thread) site() *callSite {
	if n := len(th.stack); n > 0 {
		return th.stack[n-1].site
	}
	return noSite
}

// here is the source span of the innermost active call.
func (th *Thread) here() diag.Span { return th.site().span }

// Protect runs f and converts runtime panics into errors.
func (th *Thread) Protect(f func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			switch r := r.(type) {
			case *RuntimeError:
				err = r
			case *ExitRequest:
				panic(r)
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

// CallFunc calls a top-level function by its declaration info.
func (th *Thread) CallFunc(fi *types.FuncInfo, args []Value) Value {
	fn := th.in.fns[fi]
	return th.callFn(fn, args, &callSite{name: fi.QualName(), span: fi.Decl.NameSpan})
}

// ---- frames ----

type ctl int8

const (
	ctlNone ctl = iota
	ctlReturn
	ctlBreak
	ctlContinue
)

// frame holds the local variables of one function or lambda activation.
// Every variable has a fixed slot chosen at compile time.
type frame struct {
	th    *Thread
	slots []Value
	ret   Value
	ctl   ctl
}

// Box is a heap cell for a `var` that a lambda captures, so that the lambda
// and its creator share assignments.
type Box struct{ v Value }

type (
	code  func(f *frame) Value
	bcode func(f *frame) bool
	icode func(f *frame) int64
	fcode func(f *frame) float64
)

func (th *Thread) newFrame(n int) *frame {
	if k := len(th.free); k > 0 {
		f := th.free[k-1]
		th.free = th.free[:k-1]
		if cap(f.slots) >= n {
			f.slots = f.slots[:n]
		} else {
			f.slots = make([]Value, n)
		}
		return f
	}
	return &frame{th: th, slots: make([]Value, n)}
}

func (th *Thread) freeFrame(f *frame) {
	clear(f.slots)
	f.ret = nil
	f.ctl = ctlNone
	th.free = append(th.free, f)
}

// ---- calls ----

// fnCode is a compiled function.
type fnCode struct {
	name    string
	fi      *types.FuncInfo
	nparams int
	nslots  int
	body    code
	native  *Native
	effects types.EffSet

	requires   []contract
	ensures    []contract
	resultSlot int
}

// contract is a compiled requires/ensures clause.
type contract struct {
	test bcode
	expr syntax.Expr
	l, r code // operands of a top-level comparison, re-evaluated to explain a failure
}

// Closure is a lambda together with the values it captured.
type Closure struct {
	fn   *lambdaCode
	caps []Value
}

type lambdaCode struct {
	nparams  int
	nslots   int
	capSlots []int
	body     code
}

// FuncRef is a top-level function used as a value.
type FuncRef struct {
	Info *types.FuncInfo
	fn   *fnCode
}

// Native is a function implemented in Go.
type Native struct {
	Name string
	Fn   func(th *Thread, args []Value) Value
}

func (th *Thread) push(name string, site *callSite) {
	if len(th.stack) >= maxDepth {
		th.fail("R501", site.span, "stack overflow: more than %d nested calls", maxDepth)
	}
	th.stack = append(th.stack, stackEntry{name: name, site: site})
}

func (th *Thread) pop() { th.stack = th.stack[:len(th.stack)-1] }

// callValue calls a function value (FuncRef, Closure or Native).
func (th *Thread) callValue(fv Value, args []Value, site *callSite) Value {
	switch f := fv.(type) {
	case *FuncRef:
		return th.callFn(f.fn, args, site)
	case *Closure:
		return th.callClosure(f, args, site)
	case *Native:
		return f.Fn(th, args)
	}
	th.fail("R999", site.span, "value is not callable")
	return nil
}

// callFn calls fn with positional arguments.
func (th *Thread) callFn(fn *fnCode, args []Value, site *callSite) Value {
	if fn.native != nil {
		return th.callNative(fn, args, site)
	}
	nf := th.newFrame(fn.nslots)
	copy(nf.slots, args)
	return th.invoke(fn, nf, site)
}

func (th *Thread) callNative(fn *fnCode, args []Value, site *callSite) Value {
	if fn.effects&^th.in.Granted != 0 {
		th.fail("R401", site.span, "effect `%s` was not granted to this program", (fn.effects &^ th.in.Granted).String())
	}
	th.push(fn.name, site)
	v := fn.native.Fn(th, args)
	th.pop()
	return v
}

// invoke runs fn's body in nf, whose parameter slots are already filled.
func (th *Thread) invoke(fn *fnCode, nf *frame, site *callSite) Value {
	th.push(fn.name, site)
	for i := range fn.requires {
		if !fn.requires[i].test(nf) {
			th.contractFail("requires", &fn.requires[i], nf)
		}
	}
	v := fn.body(nf)
	if nf.ctl == ctlReturn {
		v = nf.ret
	}
	if len(fn.ensures) > 0 {
		nf.ctl = ctlNone
		nf.slots[fn.resultSlot] = v
		for i := range fn.ensures {
			if !fn.ensures[i].test(nf) {
				th.contractFail("ensures", &fn.ensures[i], nf)
			}
		}
	}
	th.pop()
	th.freeFrame(nf)
	return v
}

func (th *Thread) callClosure(c *Closure, args []Value, site *callSite) Value {
	l := c.fn
	nf := th.newFrame(l.nslots)
	copy(nf.slots, args)
	for i, s := range l.capSlots {
		nf.slots[s] = c.caps[i]
	}
	th.push("lambda", site)
	v := l.body(nf)
	if nf.ctl == ctlReturn {
		v = nf.ret
	}
	th.pop()
	th.freeFrame(nf)
	return v
}

func (th *Thread) contractFail(kind string, c *contract, f *frame) {
	err := &RuntimeError{Code: "R201", Message: fmt.Sprintf("contract violated: %s %s", kind, th.src(c.expr)), Span: c.expr.Sp(), Stack: th.trace()}
	if c.l != nil {
		err.Details = []string{"left:  " + Repr(c.l(f)), "right: " + Repr(c.r(f))}
	}
	panic(err)
}

func (th *Thread) src(n syntax.Node) string {
	sp := n.Sp()
	s, ok := th.in.Sources[sp.File]
	if !ok || sp.End.Offset > len(s) || sp.Start.Offset > sp.End.Offset {
		return ""
	}
	return s[sp.Start.Offset:sp.End.Offset]
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
		body := in.compileTest(mod, t)
		th := in.NewThread(mod)
		err := th.Protect(func() {
			f := th.newFrame(body.nslots)
			body.body(f)
		})
		r := TestResult{Name: t.Name, Module: mod.Path, Span: t.Span, Passed: err == nil}
		if err != nil {
			r.Error = err.(*RuntimeError)
		}
		out = append(out, r)
	}
	return out
}
