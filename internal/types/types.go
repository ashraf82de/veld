// Package types implements Veld's static checker: name resolution, type
// inference by unification, effect checking, and match exhaustiveness.
package types

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ashraf82de/veld/internal/syntax"
)

type Type interface{ isType() }

// TCon is a named type constructor application: Int, List[Str], shapes.Shape.
type TCon struct {
	Name string    // display name ("Int", "List", "Shape", "http.Request")
	Info *TypeInfo // nil for builtins (Int, Float, Bool, Str, Unit, List, Map)
	Args []Type
}

// TFn is a function type.
type TFn struct {
	Params  []Type
	Ret     Type
	Effects EffSet
}

// TVar is a unification variable.
type TVar struct {
	ID  int
	Ref Type
	// Poison marks the type of an expression that already has an error, so
	// follow-on errors about it are suppressed.
	Poison bool
}

// TGen is a rigid generic type parameter inside a generic declaration.
type TGen struct {
	Name string
	ID   int
}

func (*TCon) isType() {}
func (*TFn) isType()  {}
func (*TVar) isType() {}
func (*TGen) isType() {}

var (
	TInt   = &TCon{Name: "Int"}
	TFloat = &TCon{Name: "Float"}
	TBool  = &TCon{Name: "Bool"}
	TStr   = &TCon{Name: "Str"}
	TUnit  = &TCon{Name: "Unit"}
)

func ListOf(t Type) Type   { return &TCon{Name: "List", Args: []Type{t}} }
func MapOf(k, v Type) Type { return &TCon{Name: "Map", Args: []Type{k, v}} }
func isBuiltinName(n string) bool {
	switch n {
	case "Int", "Float", "Bool", "Str", "Unit", "List", "Map":
		return true
	}
	return false
}

// EffSet is a bitset of effects.
type EffSet uint8

func EffectBit(name string) EffSet {
	for i, e := range syntax.Effects {
		if e == name {
			return 1 << i
		}
	}
	return 0
}

func EffectsOf(names []string) EffSet {
	var s EffSet
	for _, n := range names {
		s |= EffectBit(n)
	}
	return s
}

const AllEffects EffSet = 0xFF

func (s EffSet) Names() []string {
	var out []string
	for i, e := range syntax.Effects {
		if s&(1<<i) != 0 {
			out = append(out, e)
		}
	}
	return out
}

func (s EffSet) String() string { return strings.Join(s.Names(), ", ") }

// Prune follows bound type variables.
func Prune(t Type) Type {
	for {
		v, ok := t.(*TVar)
		if !ok || v.Ref == nil {
			return t
		}
		t = v.Ref
	}
}

// Show renders a type the way it is written in source.
func Show(t Type) string {
	t = Prune(t)
	switch t := t.(type) {
	case *TCon:
		if len(t.Args) == 0 {
			return t.Name
		}
		parts := make([]string, len(t.Args))
		for i, a := range t.Args {
			parts[i] = Show(a)
		}
		return t.Name + "[" + strings.Join(parts, ", ") + "]"
	case *TFn:
		parts := make([]string, len(t.Params))
		for i, a := range t.Params {
			parts[i] = Show(a)
		}
		s := "Fn(" + strings.Join(parts, ", ") + ") -> " + Show(t.Ret)
		if t.Effects != 0 {
			s += " uses " + t.Effects.String()
		}
		return s
	case *TVar:
		return "_"
	case *TGen:
		return t.Name
	}
	return "?"
}

func occurs(v *TVar, t Type) bool {
	t = Prune(t)
	switch t := t.(type) {
	case *TVar:
		return t == v
	case *TCon:
		for _, a := range t.Args {
			if occurs(v, a) {
				return true
			}
		}
	case *TFn:
		for _, a := range t.Params {
			if occurs(v, a) {
				return true
			}
		}
		return occurs(v, t.Ret)
	}
	return false
}

func sameCon(a, b *TCon) bool {
	if a.Info != nil || b.Info != nil {
		return a.Info == b.Info
	}
	return a.Name == b.Name
}

// Unify makes a and b equal or reports false. Function effects are not
// unified; effect compatibility is checked separately at call sites.
func Unify(a, b Type) bool {
	a, b = Prune(a), Prune(b)
	if a == b {
		return true
	}
	if va, ok := a.(*TVar); ok {
		if occurs(va, b) {
			return false
		}
		va.Ref = b
		return true
	}
	if vb, ok := b.(*TVar); ok {
		return Unify(vb, a)
	}
	switch a := a.(type) {
	case *TCon:
		bc, ok := b.(*TCon)
		if !ok || !sameCon(a, bc) || len(a.Args) != len(bc.Args) {
			return false
		}
		for i := range a.Args {
			if !Unify(a.Args[i], bc.Args[i]) {
				return false
			}
		}
		return true
	case *TFn:
		bf, ok := b.(*TFn)
		if !ok || len(a.Params) != len(bf.Params) {
			return false
		}
		for i := range a.Params {
			if !Unify(a.Params[i], bf.Params[i]) {
				return false
			}
		}
		return Unify(a.Ret, bf.Ret)
	case *TGen:
		bg, ok := b.(*TGen)
		return ok && a.ID == bg.ID
	}
	return false
}

// Subst replaces generic parameters according to m.
func Subst(t Type, m map[int]Type) Type {
	if len(m) == 0 {
		return t
	}
	t = Prune(t)
	switch t := t.(type) {
	case *TGen:
		if r, ok := m[t.ID]; ok {
			return r
		}
		return t
	case *TCon:
		if len(t.Args) == 0 {
			return t
		}
		args := make([]Type, len(t.Args))
		for i, a := range t.Args {
			args[i] = Subst(a, m)
		}
		return &TCon{Name: t.Name, Info: t.Info, Args: args}
	case *TFn:
		ps := make([]Type, len(t.Params))
		for i, a := range t.Params {
			ps[i] = Subst(a, m)
		}
		return &TFn{Params: ps, Ret: Subst(t.Ret, m), Effects: t.Effects}
	}
	return t
}

func containsFn(t Type) bool {
	t = Prune(t)
	switch t := t.(type) {
	case *TFn:
		return true
	case *TCon:
		for _, a := range t.Args {
			if containsFn(a) {
				return true
			}
		}
	}
	return false
}

func isCon(t Type, name string) bool {
	c, ok := Prune(t).(*TCon)
	return ok && c.Info == nil && c.Name == name
}

func isUnresolved(t Type) bool {
	_, ok := Prune(t).(*TVar)
	return ok
}

func isPoison(t Type) bool {
	v, ok := Prune(t).(*TVar)
	return ok && v.Poison
}

// ---- declarations ----

type FieldInfo struct {
	Name string
	Type Type
}

// TypeInfo describes a user-declared record or sum type.
type TypeInfo struct {
	Name     string
	Module   *Module
	TParams  []*TGen
	IsRecord bool
	Fields   []FieldInfo // records
	Ctors    []*CtorInfo // sum types
	Pub      bool
	Decl     syntax.Decl
}

func (ti *TypeInfo) QualName() string {
	if ti.Module == nil || ti.Module.IsPrelude || ti.Module.IsEntry {
		return ti.Name
	}
	return ti.Module.Name + "." + ti.Name
}

func (ti *TypeInfo) Field(name string) (FieldInfo, int, bool) {
	for i, f := range ti.Fields {
		if f.Name == name {
			return f, i, true
		}
	}
	return FieldInfo{}, -1, false
}

// Instantiate returns the type applied to fresh variables and the mapping.
func (c *Checker) instantiateType(ti *TypeInfo) (*TCon, map[int]Type) {
	m := map[int]Type{}
	args := make([]Type, len(ti.TParams))
	for i, g := range ti.TParams {
		v := c.fresh()
		m[g.ID] = v
		args[i] = v
	}
	return &TCon{Name: ti.QualName(), Info: ti, Args: args}, m
}

type CtorInfo struct {
	Name   string
	Type   *TypeInfo
	Fields []FieldInfo
	Index  int
}

type ParamInfo struct {
	Name string
	Type Type
}

// FuncInfo describes a top-level function.
type FuncInfo struct {
	Name    string
	Module  *Module
	TParams []*TGen
	Params  []ParamInfo
	Ret     Type
	Effects EffSet
	Pub     bool
	Extern  bool
	Decl    *syntax.FnDecl
}

func (f *FuncInfo) QualName() string {
	if f.Module == nil || f.Module.IsPrelude {
		return f.Name
	}
	return f.Module.Name + "." + f.Name
}

// Signature renders the function's signature as source.
func (f *FuncInfo) Signature() string {
	var b strings.Builder
	b.WriteString("fn ")
	b.WriteString(f.Name)
	if len(f.TParams) > 0 {
		names := make([]string, len(f.TParams))
		for i, g := range f.TParams {
			names[i] = g.Name
		}
		b.WriteString("[" + strings.Join(names, ", ") + "]")
	}
	parts := make([]string, len(f.Params))
	for i, p := range f.Params {
		parts[i] = p.Name + ": " + Show(p.Type)
	}
	b.WriteString("(" + strings.Join(parts, ", ") + ") -> " + Show(f.Ret))
	if f.Effects != 0 {
		b.WriteString(" uses " + f.Effects.String())
	}
	return b.String()
}

// Module is a checked source file.
type Module struct {
	Path      string // "std.list", "util.text", or the entry file stem
	Name      string // last path segment, used for qualified access
	File      *syntax.File
	IsStd     bool
	IsPrelude bool
	IsEntry   bool
	Imports   map[string]*Module // by local name
	Broken    map[string]bool    // imports that failed to load (already reported)
	Types     map[string]*TypeInfo
	Funcs     map[string]*FuncInfo
	Ctors     map[string]*CtorInfo
	Tests     []*syntax.TestDecl
}

func newModule(path string, f *syntax.File) *Module {
	parts := strings.Split(path, ".")
	return &Module{Path: path, Name: parts[len(parts)-1], File: f, Imports: map[string]*Module{}, Broken: map[string]bool{},
		Types: map[string]*TypeInfo{}, Funcs: map[string]*FuncInfo{}, Ctors: map[string]*CtorInfo{}}
}

// SortedFuncs returns the module's functions in declaration order.
func (m *Module) SortedFuncs() []*FuncInfo {
	var out []*FuncInfo
	for _, f := range m.Funcs {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Decl.Span.Start.Offset < out[j].Decl.Span.Start.Offset })
	return out
}

func fmtList(xs []string) string {
	switch len(xs) {
	case 0:
		return ""
	case 1:
		return xs[0]
	}
	return strings.Join(xs[:len(xs)-1], ", ") + " and " + xs[len(xs)-1]
}

var _ = fmt.Sprintf
