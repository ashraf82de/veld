// Package interp is Veld's reference tree-walking interpreter.
package interp

import (
	"hash/maphash"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/ashraf82de/veld/internal/pds"
	"github.com/ashraf82de/veld/internal/syntax"
	"github.com/ashraf82de/veld/internal/types"
)

// Value is a runtime value: int64, float64, bool, string, Unit, List, Map,
// *Record, *Variant, *Closure, *FuncRef or *Native.
type Value = any

type UnitT struct{}

var Unit = UnitT{}

// List is an immutable persistent vector.
type List = *pds.Vec

// Map is an immutable, insertion-ordered persistent hash map.
type Map = *pds.Map

var mapCfg = &pds.Config{Hash: HashOf, Equal: Equal}

// NewMap returns an empty map.
func NewMap() Map { return pds.NewMap(mapCfg) }

// NewList copies xs into a list.
func NewList(xs []Value) List { return pds.FromSlice(xs) }

// EmptyList is the list with no elements.
func EmptyList() List { return pds.Empty() }

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
		v.Range(func(i int, x Value) bool {
			if i > 0 {
				b.WriteString(", ")
			}
			writeRepr(b, x)
			return true
		})
		b.WriteByte(']')
	case Map:
		b.WriteByte('{')
		first := true
		v.Range(func(k, val Value) bool {
			if !first {
				b.WriteString(", ")
			}
			first = false
			writeRepr(b, k)
			b.WriteString(": ")
			writeRepr(b, val)
			return true
		})
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
		if !ok || a.Len() != bl.Len() {
			return false
		}
		eq := true
		a.Range(func(i int, x Value) bool {
			eq = Equal(x, bl.Get(i))
			return eq
		})
		return eq
	case Map:
		bm, ok := b.(Map)
		if !ok || a.Len() != bm.Len() {
			return false
		}
		eq := true
		a.Range(func(k, v Value) bool {
			w, found := bm.Get(k)
			eq = found && Equal(v, w)
			return eq
		})
		return eq
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
	out := xs.ToSlice()
	if key == nil {
		sort.SliceStable(out, func(i, j int) bool { return Compare(out[i], out[j]) < 0 })
		return NewList(out)
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
	res := make([]Value, len(out))
	for i, k := range idx {
		res[i] = out[k]
	}
	return NewList(res)
}

var hashSeed = maphash.MakeSeed()

func mix(h uint64) uint64 {
	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	h *= 0xc4ceb9fe1a85ec53
	h ^= h >> 33
	return h
}

// HashOf is a structural hash consistent with Equal.
func HashOf(v Value) uint64 {
	switch v := v.(type) {
	case int64:
		return mix(uint64(v))
	case float64:
		if v == 0 {
			return mix(0)
		}
		return mix(math.Float64bits(v))
	case bool:
		if v {
			return 0x9e3779b97f4a7c15
		}
		return 0x7f4a7c159e3779b9
	case string:
		return maphash.String(hashSeed, v)
	case UnitT:
		return 1
	case List:
		h := uint64(0x1234567)
		v.Range(func(_ int, x Value) bool {
			h = mix(h*31 + HashOf(x))
			return true
		})
		return h
	case Map:
		// Order-independent, because Equal ignores insertion order.
		var h uint64 = 0x7654321
		v.Range(func(k, val Value) bool {
			h += mix(HashOf(k)*31 + HashOf(val))
			return true
		})
		return h
	case *Record:
		h := maphash.String(hashSeed, v.Type.Name)
		for _, f := range v.Fields {
			h = mix(h*31 + HashOf(f))
		}
		return h
	case *Variant:
		h := maphash.String(hashSeed, v.Ctor.Name)
		for _, f := range v.Fields {
			h = mix(h*31 + HashOf(f))
		}
		return h
	}
	return 0
}
