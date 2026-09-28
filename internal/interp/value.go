// Package interp is Veld's reference tree-walking interpreter.
package interp

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/ashraf82de/veld/internal/syntax"
	"github.com/ashraf82de/veld/internal/types"
)

// Value is a runtime value: int64, float64, bool, string, Unit, List, *Map,
// *Record, *Variant, *Closure, *FuncRef or *Native.
type Value any

type UnitT struct{}

var Unit = UnitT{}

type List []Value

// Map is an immutable, insertion-ordered map.
type Map struct {
	Keys  []Value
	Vals  []Value
	index map[string]int
}

func NewMap() *Map { return &Map{index: map[string]int{}} }

func (m *Map) Get(k Value) (Value, bool) {
	i, ok := m.index[KeyOf(k)]
	if !ok {
		return nil, false
	}
	return m.Vals[i], true
}

// Put returns a new map with k set to v.
func (m *Map) Put(k, v Value) *Map {
	key := KeyOf(k)
	out := &Map{index: make(map[string]int, len(m.index)+1)}
	out.Keys = append(make([]Value, 0, len(m.Keys)+1), m.Keys...)
	out.Vals = append(make([]Value, 0, len(m.Vals)+1), m.Vals...)
	for kk, i := range m.index {
		out.index[kk] = i
	}
	if i, ok := out.index[key]; ok {
		out.Vals[i] = v
		return out
	}
	out.index[key] = len(out.Keys)
	out.Keys = append(out.Keys, k)
	out.Vals = append(out.Vals, v)
	return out
}

// putInPlace mutates m; only for maps under construction.
func (m *Map) putInPlace(k, v Value) {
	key := KeyOf(k)
	if i, ok := m.index[key]; ok {
		m.Vals[i] = v
		return
	}
	m.index[key] = len(m.Keys)
	m.Keys = append(m.Keys, k)
	m.Vals = append(m.Vals, v)
}

func (m *Map) Remove(k Value) *Map {
	key := KeyOf(k)
	if _, ok := m.index[key]; !ok {
		return m
	}
	out := NewMap()
	for i, kk := range m.Keys {
		if KeyOf(kk) != key {
			out.putInPlace(kk, m.Vals[i])
		}
	}
	return out
}

func KeyOf(v Value) string { return Repr(v) }

type Record struct {
	Type   *types.TypeInfo
	Fields []Value
}

type Variant struct {
	Ctor   *types.CtorInfo
	Fields []Value
}

type Closure struct {
	Params []string
	Body   *syntax.Block
	Env    *Env
	Mod    *types.Module
}

type FuncRef struct {
	Info *types.FuncInfo
}

type Native struct {
	Name string
	Fn   func(th *Thread, args []Value) Value
}

// ---- display ----

// Show renders a value for string interpolation: strings are inserted
// verbatim, everything else as Repr.
func Show(v Value) string {
	if s, ok := v.(string); ok {
		return s
	}
	return Repr(v)
}

// Repr renders a value as Veld source-like text.
func Repr(v Value) string {
	var b strings.Builder
	writeRepr(&b, v)
	return b.String()
}

func FormatFloat(f float64) string {
	if math.IsInf(f, 1) {
		return "inf"
	}
	if math.IsInf(f, -1) {
		return "-inf"
	}
	if math.IsNaN(f) {
		return "nan"
	}
	var s string
	if a := math.Abs(f); a != 0 && (a < 1e-4 || a >= 1e16) {
		s = strconv.FormatFloat(f, 'g', -1, 64)
	} else {
		s = strconv.FormatFloat(f, 'f', -1, 64)
	}
	if !strings.ContainsAny(s, ".e") {
		s += ".0"
	}
	return s
}

func writeRepr(b *strings.Builder, v Value) {
	switch v := v.(type) {
	case int64:
		b.WriteString(strconv.FormatInt(v, 10))
	case float64:
		b.WriteString(FormatFloat(v))
	case bool:
		b.WriteString(strconv.FormatBool(v))
	case string:
		b.WriteString(quote(v))
	case UnitT:
		b.WriteString("Unit")
	case List:
		b.WriteByte('[')
		for i, x := range v {
			if i > 0 {
				b.WriteString(", ")
			}
			writeRepr(b, x)
		}
		b.WriteByte(']')
	case *Map:
		b.WriteByte('{')
		for i, k := range v.Keys {
			if i > 0 {
				b.WriteString(", ")
			}
			writeRepr(b, k)
			b.WriteString(": ")
			writeRepr(b, v.Vals[i])
		}
		b.WriteByte('}')
	case *Record:
		b.WriteString(v.Type.Name)
		b.WriteByte('{')
		for i, f := range v.Type.Fields {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(f.Name)
			b.WriteString(": ")
			writeRepr(b, v.Fields[i])
		}
		b.WriteByte('}')
	case *Variant:
		b.WriteString(v.Ctor.Name)
		if len(v.Fields) > 0 {
			b.WriteByte('(')
			for i, f := range v.Fields {
				if i > 0 {
					b.WriteString(", ")
				}
				writeRepr(b, f)
			}
			b.WriteByte(')')
		}
	case *Closure:
		b.WriteString("<fn>")
	case *FuncRef:
		b.WriteString("<fn " + v.Info.Name + ">")
	case *Native:
		b.WriteString("<fn " + v.Name + ">")
	default:
		b.WriteString("<?>")
	}
}

func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		case '$':
			b.WriteString(`\$`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// Equal is structural equality.
func Equal(a, b Value) bool {
	switch a := a.(type) {
	case int64, float64, bool, string, UnitT:
		return a == b
	case List:
		bl, ok := b.(List)
		if !ok || len(a) != len(bl) {
			return false
		}
		for i := range a {
			if !Equal(a[i], bl[i]) {
				return false
			}
		}
		return true
	case *Map:
		bm, ok := b.(*Map)
		if !ok || len(a.Keys) != len(bm.Keys) {
			return false
		}
		for i, k := range a.Keys {
			v, ok := bm.Get(k)
			if !ok || !Equal(a.Vals[i], v) {
				return false
			}
		}
		return true
	case *Record:
		br, ok := b.(*Record)
		if !ok || a.Type != br.Type {
			return false
		}
		for i := range a.Fields {
			if !Equal(a.Fields[i], br.Fields[i]) {
				return false
			}
		}
		return true
	case *Variant:
		bv, ok := b.(*Variant)
		if !ok || a.Ctor != bv.Ctor {
			return false
		}
		for i := range a.Fields {
			if !Equal(a.Fields[i], bv.Fields[i]) {
				return false
			}
		}
		return true
	}
	return false
}

// Compare orders Int, Float and Str values.
func Compare(a, b Value) int {
	switch a := a.(type) {
	case int64:
		bi := b.(int64)
		switch {
		case a < bi:
			return -1
		case a > bi:
			return 1
		}
		return 0
	case float64:
		bf := b.(float64)
		switch {
		case a < bf:
			return -1
		case a > bf:
			return 1
		}
		return 0
	case string:
		return strings.Compare(a, b.(string))
	}
	return 0
}

func sortValues(xs List, key func(Value) Value) List {
	out := append(List{}, xs...)
	if key == nil {
		sort.SliceStable(out, func(i, j int) bool { return Compare(out[i], out[j]) < 0 })
		return out
	}
	keys := make([]Value, len(out))
	for i, x := range out {
		keys[i] = key(x)
	}
	idx := make([]int, len(out))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(i, j int) bool { return Compare(keys[idx[i]], keys[idx[j]]) < 0 })
	res := make(List, len(out))
	for i, k := range idx {
		res[i] = out[k]
	}
	return res
}
