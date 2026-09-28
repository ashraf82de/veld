package interp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/ashraf82de/veld/internal/types"
)

var stdin = bufio.NewReader(os.Stdin)
var stdinMu sync.Mutex

func (in *Interp) some(v Value) Value {
	return &Variant{Ctor: in.Prog.Prelude.Ctors["Some"], Fields: []Value{v}}
}
func (in *Interp) none() Value { return &Variant{Ctor: in.Prog.Prelude.Ctors["None"]} }
func (in *Interp) ok(v Value) Value {
	return &Variant{Ctor: in.Prog.Prelude.Ctors["Ok"], Fields: []Value{v}}
}
func (in *Interp) err(msg string) Value {
	return &Variant{Ctor: in.Prog.Prelude.Ctors["Err"], Fields: []Value{msg}}
}
func (in *Interp) ctor(name string, fields ...Value) Value {
	return &Variant{Ctor: in.Prog.Prelude.Ctors[name], Fields: fields}
}

func (th *Thread) nativeFail(format string, args ...any) {
	th.fail("R300", th.here(), format, args...)
}

func runes(s string) []rune { return []rune(s) }

func clamp(i, lo, hi int64) int64 {
	if i < lo {
		return lo
	}
	if i > hi {
		return hi
	}
	return i
}

func natives() map[string]*Native {
	m := map[string]*Native{}
	def := func(name string, f func(th *Thread, a []Value) Value) { m[name] = &Native{Name: name, Fn: f} }

	// ---- prelude ----
	def("panic", func(th *Thread, a []Value) Value {
		th.fail("R302", th.here(), "panic: %s", a[0].(string))
		return nil
	})

	// ---- io ----
	def("io.print", func(th *Thread, a []Value) Value { th.in.out(a[0].(string) + "\n"); return Unit })
	def("io.write", func(th *Thread, a []Value) Value { th.in.out(a[0].(string)); return Unit })
	def("io.eprint", func(th *Thread, a []Value) Value { th.in.errOut(a[0].(string) + "\n"); return Unit })
	def("io.read_line", func(th *Thread, a []Value) Value {
		stdinMu.Lock()
		defer stdinMu.Unlock()
		line, err := stdin.ReadString('\n')
		if err != nil && line == "" {
			return th.in.none()
		}
		return th.in.some(strings.TrimRight(line, "\r\n"))
	})
	def("io.read_all", func(th *Thread, a []Value) Value {
		stdinMu.Lock()
		defer stdinMu.Unlock()
		b, _ := io.ReadAll(stdin)
		return string(b)
	})

	// ---- str ----
	def("str.len", func(th *Thread, a []Value) Value { return int64(utf8.RuneCountInString(a[0].(string))) })
	def("str.split", func(th *Thread, a []Value) Value {
		sep := a[1].(string)
		if sep == "" {
			th.nativeFail("str.split: separator must not be empty (use str.chars)")
		}
		return strList(strings.Split(a[0].(string), sep))
	})
	def("str.join", func(th *Thread, a []Value) Value {
		l := a[0].(List)
		parts := make([]string, 0, l.Len())
		l.Range(func(_ int, x Value) bool {
			parts = append(parts, x.(string))
			return true
		})
		return strings.Join(parts, a[1].(string))
	})
	def("str.lines", func(th *Thread, a []Value) Value {
		s := strings.ReplaceAll(a[0].(string), "\r\n", "\n")
		s = strings.TrimSuffix(s, "\n")
		if s == "" {
			return EmptyList()
		}
		return strList(strings.Split(s, "\n"))
	})
	def("str.words", func(th *Thread, a []Value) Value { return strList(strings.Fields(a[0].(string))) })
	def("str.trim", func(th *Thread, a []Value) Value { return strings.TrimSpace(a[0].(string)) })
	def("str.trim_start", func(th *Thread, a []Value) Value { return strings.TrimLeftFunc(a[0].(string), unicode.IsSpace) })
	def("str.trim_end", func(th *Thread, a []Value) Value { return strings.TrimRightFunc(a[0].(string), unicode.IsSpace) })
	def("str.upper", func(th *Thread, a []Value) Value { return strings.ToUpper(a[0].(string)) })
	def("str.lower", func(th *Thread, a []Value) Value { return strings.ToLower(a[0].(string)) })
	def("str.contains", func(th *Thread, a []Value) Value { return strings.Contains(a[0].(string), a[1].(string)) })
	def("str.starts_with", func(th *Thread, a []Value) Value { return strings.HasPrefix(a[0].(string), a[1].(string)) })
	def("str.ends_with", func(th *Thread, a []Value) Value { return strings.HasSuffix(a[0].(string), a[1].(string)) })
	def("str.replace", func(th *Thread, a []Value) Value {
		if a[1].(string) == "" {
			th.nativeFail("str.replace: `old` must not be empty")
		}
		return strings.ReplaceAll(a[0].(string), a[1].(string), a[2].(string))
	})
	def("str.slice", func(th *Thread, a []Value) Value {
		r := runes(a[0].(string))
		n := int64(len(r))
		s, e := clamp(a[1].(int64), 0, n), clamp(a[2].(int64), 0, n)
		if e < s {
			return ""
		}
		return string(r[s:e])
	})
	def("str.index_of", func(th *Thread, a []Value) Value {
		i := strings.Index(a[0].(string), a[1].(string))
		if i < 0 {
			return th.in.none()
		}
		return th.in.some(int64(utf8.RuneCountInString(a[0].(string)[:i])))
	})
	def("str.count", func(th *Thread, a []Value) Value {
		if a[1].(string) == "" {
			th.nativeFail("str.count: `part` must not be empty")
		}
		return int64(strings.Count(a[0].(string), a[1].(string)))
	})
	def("str.to_int", func(th *Thread, a []Value) Value {
		i, err := strconv.ParseInt(strings.TrimSpace(a[0].(string)), 10, 64)
		if err != nil {
			return th.in.none()
		}
		return th.in.some(i)
	})
	def("str.to_float", func(th *Thread, a []Value) Value {
		f, err := strconv.ParseFloat(strings.TrimSpace(a[0].(string)), 64)
		if err != nil {
			return th.in.none()
		}
		return th.in.some(f)
	})
	def("str.chars", func(th *Thread, a []Value) Value {
		var out []Value
		for _, r := range a[0].(string) {
			out = append(out, string(r))
		}
		return NewList(out)
	})
	def("str.code", func(th *Thread, a []Value) Value {
		s := a[0].(string)
		if s == "" {
			return th.in.none()
		}
		r, _ := utf8.DecodeRuneInString(s)
		return th.in.some(int64(r))
	})
	def("str.from_code", func(th *Thread, a []Value) Value { return string(rune(a[0].(int64))) })
	def("str.repeat", func(th *Thread, a []Value) Value {
		n := a[1].(int64)
		if n < 0 {
			th.nativeFail("str.repeat: times must be >= 0, got %d", n)
		}
		return strings.Repeat(a[0].(string), int(n))
	})
	def("str.reverse", func(th *Thread, a []Value) Value {
		r := runes(a[0].(string))
		for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
			r[i], r[j] = r[j], r[i]
		}
		return string(r)
	})
	pad := func(left bool) func(th *Thread, a []Value) Value {
		return func(th *Thread, a []Value) Value {
			s, w, fill := a[0].(string), a[1].(int64), a[2].(string)
			if utf8.RuneCountInString(fill) != 1 {
				th.nativeFail("pad: fill must be exactly one character, got %q", fill)
			}
			n := int(w) - utf8.RuneCountInString(s)
			if n <= 0 {
				return s
			}
			if left {
				return strings.Repeat(fill, n) + s
			}
			return s + strings.Repeat(fill, n)
		}
	}
	def("str.pad_left", pad(true))
	def("str.pad_right", pad(false))
	def("str.fixed", func(th *Thread, a []Value) Value {
		return strconv.FormatFloat(a[0].(float64), 'f', int(clamp(a[1].(int64), 0, 20)), 64)
	})

	// ---- list ----
	def("list.len", func(th *Thread, a []Value) Value { return int64(a[0].(List).Len()) })
	def("list.get", func(th *Thread, a []Value) Value {
		l, i := a[0].(List), a[1].(int64)
		if i < 0 || i >= int64(l.Len()) {
			return th.in.none()
		}
		return th.in.some(l.Get(int(i)))
	})
	def("list.push", func(th *Thread, a []Value) Value { return a[0].(List).Push(a[1]) })
	def("list.prepend", func(th *Thread, a []Value) Value {
		return NewList(append([]Value{a[1]}, a[0].(List).ToSlice()...))
	})
	def("list.set_at", func(th *Thread, a []Value) Value {
		l, i := a[0].(List), a[1].(int64)
		if i < 0 || i >= int64(l.Len()) {
			th.nativeFail("list.set_at: index %d out of range for list of length %d", i, l.Len())
		}
		return l.Set(int(i), a[2])
	})
	def("list.remove_at", func(th *Thread, a []Value) Value {
		l, i := a[0].(List), a[1].(int64)
		if i < 0 || i >= int64(l.Len()) {
			th.nativeFail("list.remove_at: index %d out of range for list of length %d", i, l.Len())
		}
		return l.Slice(0, int(i)).Concat(l.Slice(int(i)+1, l.Len()))
	})
	def("list.range", func(th *Thread, a []Value) Value {
		s, e := a[0].(int64), a[1].(int64)
		if e-s > 50_000_000 {
			th.nativeFail("list.range: range of %d elements is too large", e-s)
		}
		if e <= s {
			return EmptyList()
		}
		out := make([]Value, 0, e-s)
		for i := s; i < e; i++ {
			out = append(out, i)
		}
		return NewList(out)
	})
	def("list.map", func(th *Thread, a []Value) Value {
		l := a[0].(List)
		out := make([]Value, 0, l.Len())
		l.Range(func(_ int, x Value) bool {
			out = append(out, th.callValue(a[1], []Value{x}, th.site()))
			return true
		})
		return NewList(out)
	})
	def("list.map_indexed", func(th *Thread, a []Value) Value {
		l := a[0].(List)
		out := make([]Value, 0, l.Len())
		l.Range(func(i int, x Value) bool {
			out = append(out, th.callValue(a[1], []Value{int64(i), x}, th.site()))
			return true
		})
		return NewList(out)
	})
	def("list.filter", func(th *Thread, a []Value) Value {
		var out []Value
		a[0].(List).Range(func(_ int, x Value) bool {
			if th.callValue(a[1], []Value{x}, th.site()) == true {
				out = append(out, x)
			}
			return true
		})
		return NewList(out)
	})
	def("list.fold", func(th *Thread, a []Value) Value {
		acc := a[1]
		a[0].(List).Range(func(_ int, x Value) bool {
			acc = th.callValue(a[2], []Value{acc, x}, th.site())
			return true
		})
		return acc
	})
	def("list.sort", func(th *Thread, a []Value) Value { return sortValues(a[0].(List), nil) })
	def("list.sort_by", func(th *Thread, a []Value) Value {
		return sortValues(a[0].(List), func(v Value) Value { return th.callValue(a[1], []Value{v}, th.site()) })
	})
	def("list.reverse", func(th *Thread, a []Value) Value {
		xs := a[0].(List).ToSlice()
		for i, j := 0, len(xs)-1; i < j; i, j = i+1, j-1 {
			xs[i], xs[j] = xs[j], xs[i]
		}
		return NewList(xs)
	})
	def("list.take", func(th *Thread, a []Value) Value {
		l := a[0].(List)
		return l.Slice(0, int(clamp(a[1].(int64), 0, int64(l.Len()))))
	})
	def("list.drop", func(th *Thread, a []Value) Value {
		l := a[0].(List)
		return l.Slice(int(clamp(a[1].(int64), 0, int64(l.Len()))), l.Len())
	})
	def("list.slice", func(th *Thread, a []Value) Value {
		l := a[0].(List)
		n := int64(l.Len())
		s, e := clamp(a[1].(int64), 0, n), clamp(a[2].(int64), 0, n)
		if e < s {
			return EmptyList()
		}
		return l.Slice(int(s), int(e))
	})
	def("list.unique", func(th *Thread, a []Value) Value {
		seen := NewMap()
		var out []Value
		a[0].(List).Range(func(_ int, x Value) bool {
			if !seen.Has(x) {
				seen = seen.Put(x, true)
				out = append(out, x)
			}
			return true
		})
		return NewList(out)
	})
	def("list.repeat", func(th *Thread, a []Value) Value {
		n := a[1].(int64)
		if n < 0 {
			th.nativeFail("list.repeat: times must be >= 0, got %d", n)
		}
		out := make([]Value, n)
		for i := range out {
			out[i] = a[0]
		}
		return NewList(out)
	})
	extreme := func(sign int) func(th *Thread, a []Value) Value {
		return func(th *Thread, a []Value) Value {
			l := a[0].(List)
			if l.Len() == 0 {
				return th.in.none()
			}
			best := l.Get(0)
			l.Range(func(_ int, x Value) bool {
				if Compare(x, best)*sign > 0 {
					best = x
				}
				return true
			})
			return th.in.some(best)
		}
	}
	def("list.max", extreme(1))
	def("list.min", extreme(-1))
	def("list.chunks", func(th *Thread, a []Value) Value {
		l, n := a[0].(List), a[1].(int64)
		if n <= 0 {
			th.nativeFail("list.chunks: size must be > 0, got %d", n)
		}
		var out []Value
		for i := int64(0); i < int64(l.Len()); i += n {
			out = append(out, l.Slice(int(i), int(min(i+n, int64(l.Len())))))
		}
		return NewList(out)
	})

	// ---- map ----
	def("map.len", func(th *Thread, a []Value) Value { return int64(a[0].(Map).Len()) })
	def("map.get", func(th *Thread, a []Value) Value {
		if v, ok := a[0].(Map).Get(a[1]); ok {
			return th.in.some(v)
		}
		return th.in.none()
	})
	def("map.has", func(th *Thread, a []Value) Value { return a[0].(Map).Has(a[1]) })
	def("map.put", func(th *Thread, a []Value) Value { return a[0].(Map).Put(a[1], a[2]) })
	def("map.remove", func(th *Thread, a []Value) Value { return a[0].(Map).Remove(a[1]) })
	def("map.keys", func(th *Thread, a []Value) Value { return NewList(a[0].(Map).Keys()) })
	def("map.values", func(th *Thread, a []Value) Value { return NewList(a[0].(Map).Values()) })
	def("map.merge", func(th *Thread, a []Value) Value {
		out := a[0].(Map)
		a[1].(Map).Range(func(k, v Value) bool {
			out = out.Put(k, v)
			return true
		})
		return out
	})

	// ---- math ----
	f1 := func(name string, f func(float64) float64) {
		def(name, func(th *Thread, a []Value) Value { return f(a[0].(float64)) })
	}
	f1("math.abs_float", math.Abs)
	f1("math.sqrt", math.Sqrt)
	f1("math.log", math.Log)
	f1("math.exp", math.Exp)
	f1("math.sin", math.Sin)
	f1("math.cos", math.Cos)
	def("math.min_float", func(th *Thread, a []Value) Value { return math.Min(a[0].(float64), a[1].(float64)) })
	def("math.max_float", func(th *Thread, a []Value) Value { return math.Max(a[0].(float64), a[1].(float64)) })
	def("math.pow", func(th *Thread, a []Value) Value { return math.Pow(a[0].(float64), a[1].(float64)) })
	def("math.pi", func(th *Thread, a []Value) Value { return math.Pi })
	def("math.to_float", func(th *Thread, a []Value) Value { return float64(a[0].(int64)) })
	toInt := func(name string, f func(float64) float64) {
		def(name, func(th *Thread, a []Value) Value {
			x := f(a[0].(float64))
			if math.IsNaN(x) || x >= 9.223372036854775807e18 || x < -9.223372036854775808e18 {
				th.nativeFail("%s: %s does not fit in an Int", name, FormatFloat(a[0].(float64)))
			}
			return int64(x)
		})
	}
	toInt("math.floor", math.Floor)
	toInt("math.ceil", math.Ceil)
	toInt("math.round", math.Round)
	def("math.pow_int", func(th *Thread, a []Value) Value {
		b, e := a[0].(int64), a[1].(int64)
		if e < 0 {
			th.nativeFail("math.pow_int: exponent must be >= 0, got %d", e)
		}
		r := int64(1)
		for i := int64(0); i < e; i++ {
			if b != 0 && (r*b)/b != r {
				th.fail("R101", th.here(), "integer overflow in math.pow_int")
			}
			r *= b
		}
		return r
	})

	// ---- fs ----
	def("fs.read", func(th *Thread, a []Value) Value {
		b, err := os.ReadFile(a[0].(string))
		if err != nil {
			return th.in.err(err.Error())
		}
		return th.in.ok(string(b))
	})
	def("fs.write", func(th *Thread, a []Value) Value {
		if err := os.WriteFile(a[0].(string), []byte(a[1].(string)), 0o644); err != nil {
			return th.in.err(err.Error())
		}
		return th.in.ok(Unit)
	})
	def("fs.append", func(th *Thread, a []Value) Value {
		f, err := os.OpenFile(a[0].(string), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return th.in.err(err.Error())
		}
		defer f.Close()
		if _, err := f.WriteString(a[1].(string)); err != nil {
			return th.in.err(err.Error())
		}
		return th.in.ok(Unit)
	})
	def("fs.exists", func(th *Thread, a []Value) Value { _, err := os.Stat(a[0].(string)); return err == nil })
	def("fs.list_dir", func(th *Thread, a []Value) Value {
		es, err := os.ReadDir(a[0].(string))
		if err != nil {
			return th.in.err(err.Error())
		}
		var names []string
		for _, e := range es {
			names = append(names, e.Name())
		}
		sort.Strings(names)
		return th.in.ok(strList(names))
	})
	def("fs.make_dir", func(th *Thread, a []Value) Value {
		if err := os.MkdirAll(a[0].(string), 0o755); err != nil {
			return th.in.err(err.Error())
		}
		return th.in.ok(Unit)
	})
	def("fs.remove", func(th *Thread, a []Value) Value {
		if err := os.Remove(a[0].(string)); err != nil {
			return th.in.err(err.Error())
		}
		return th.in.ok(Unit)
	})

	// ---- env / time / rand ----
	def("env.args", func(th *Thread, a []Value) Value { return strList(th.in.Args) })
	def("env.get", func(th *Thread, a []Value) Value {
		v, ok := os.LookupEnv(a[0].(string))
		if !ok {
			return th.in.none()
		}
		return th.in.some(v)
	})
	def("env.exit", func(th *Thread, a []Value) Value { panic(&ExitRequest{Code: int(a[0].(int64))}) })
	def("time.now_ms", func(th *Thread, a []Value) Value { return time.Now().UnixMilli() })
	def("time.sleep_ms", func(th *Thread, a []Value) Value {
		time.Sleep(time.Duration(a[0].(int64)) * time.Millisecond)
		return Unit
	})
	def("rand.int", func(th *Thread, a []Value) Value {
		lo, hi := a[0].(int64), a[1].(int64)
		if hi <= lo {
			th.nativeFail("rand.int: high (%d) must be greater than low (%d)", hi, lo)
		}
		return lo + rand.Int63n(hi-lo)
	})
	def("rand.float", func(th *Thread, a []Value) Value { return rand.Float64() })
	def("rand.shuffle", func(th *Thread, a []Value) Value {
		out := a[0].(List).ToSlice()
		rand.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
		return NewList(out)
	})

	// ---- json ----
	def("json.parse", func(th *Thread, a []Value) Value {
		dec := json.NewDecoder(strings.NewReader(a[0].(string)))
		dec.UseNumber()
		v, err := th.decodeJSON(dec)
		if err != nil {
			return th.in.err("invalid JSON: " + err.Error())
		}
		if _, err := dec.Token(); err != io.EOF {
			return th.in.err("invalid JSON: unexpected data after the value")
		}
		return th.in.ok(v)
	})
	def("json.encode", func(th *Thread, a []Value) Value {
		var b bytes.Buffer
		encodeJSON(&b, a[0], "", "")
		return b.String()
	})
	def("json.pretty", func(th *Thread, a []Value) Value {
		var b bytes.Buffer
		encodeJSON(&b, a[0], "  ", "")
		return b.String()
	})

	// ---- http ----
	def("http.serve", func(th *Thread, a []Value) Value { return th.serve(a[0].(int64), a[1]) })
	def("http.get", func(th *Thread, a []Value) Value {
		resp, err := httpClient.Get(a[0].(string))
		return th.httpResult(resp, err)
	})
	def("http.post", func(th *Thread, a []Value) Value {
		resp, err := httpClient.Post(a[0].(string), a[2].(string), strings.NewReader(a[1].(string)))
		return th.httpResult(resp, err)
	})
	return m
}

func strList(xs []string) List {
	out := make([]Value, len(xs))
	for i, x := range xs {
		out[i] = x
	}
	return NewList(out)
}

func (in *Interp) out(s string) {
	in.outMu.Lock()
	defer in.outMu.Unlock()
	in.Stdout(s)
}

func (in *Interp) errOut(s string) {
	in.outMu.Lock()
	defer in.outMu.Unlock()
	in.Stderr(s)
}

// ---- JSON ----

func (th *Thread) decodeJSON(dec *json.Decoder) (Value, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	in := th.in
	switch t := tok.(type) {
	case nil:
		return in.ctor("JNull"), nil
	case bool:
		return in.ctor("JBool", t), nil
	case json.Number:
		f, err := t.Float64()
		if err != nil {
			return nil, err
		}
		return in.ctor("JNum", f), nil
	case string:
		return in.ctor("JStr", t), nil
	case json.Delim:
		switch t {
		case '[':
			var elems []Value
			for dec.More() {
				v, err := th.decodeJSON(dec)
				if err != nil {
					return nil, err
				}
				elems = append(elems, v)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return in.ctor("JArr", NewList(elems)), nil
		case '{':
			m := NewMap()
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				v, err := th.decodeJSON(dec)
				if err != nil {
					return nil, err
				}
				m = m.Put(kt.(string), v)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return in.ctor("JObj", m), nil
		}
	}
	return nil, fmt.Errorf("unexpected token %v", tok)
}

func encodeJSON(b *bytes.Buffer, v Value, indent, cur string) {
	vv := v.(*Variant)
	nl := func(level string) {
		if indent != "" {
			b.WriteByte('\n')
			b.WriteString(level)
		}
	}
	switch vv.Ctor.Name {
	case "JNull":
		b.WriteString("null")
	case "JBool":
		b.WriteString(strconv.FormatBool(vv.Fields[0].(bool)))
	case "JNum":
		f := vv.Fields[0].(float64)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			b.WriteString("null")
		} else if f == math.Trunc(f) && math.Abs(f) < 1e15 {
			b.WriteString(strconv.FormatInt(int64(f), 10))
		} else {
			b.WriteString(strconv.FormatFloat(f, 'g', -1, 64))
		}
	case "JStr":
		s, _ := json.Marshal(vv.Fields[0].(string))
		b.Write(s)
	case "JArr":
		items := vv.Fields[0].(List)
		b.WriteByte('[')
		items.Range(func(i int, x Value) bool {
			if i > 0 {
				b.WriteByte(',')
			}
			nl(cur + indent)
			encodeJSON(b, x, indent, cur+indent)
			return true
		})
		if items.Len() > 0 {
			nl(cur)
		}
		b.WriteByte(']')
	case "JObj":
		m := vv.Fields[0].(Map)
		b.WriteByte('{')
		first := true
		m.Range(func(k, val Value) bool {
			if !first {
				b.WriteByte(',')
			}
			first = false
			nl(cur + indent)
			ks, _ := json.Marshal(k.(string))
			b.Write(ks)
			b.WriteByte(':')
			if indent != "" {
				b.WriteByte(' ')
			}
			encodeJSON(b, val, indent, cur+indent)
			return true
		})
		if m.Len() > 0 {
			nl(cur)
		}
		b.WriteByte('}')
	}
}

// ---- HTTP ----

var httpClient = &http.Client{Timeout: 30 * time.Second}

func (th *Thread) httpModule() *types.Module {
	for _, m := range th.in.Prog.Modules {
		if m.Path == "std.http" {
			return m
		}
	}
	return nil
}

func (th *Thread) httpResult(resp *http.Response, err error) Value {
	if err != nil {
		return th.in.err(err.Error())
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return th.in.err(err.Error())
	}
	hm := th.httpModule()
	headers := NewMap()
	for k, vs := range resp.Header {
		headers = headers.Put(strings.ToLower(k), strings.Join(vs, ", "))
	}
	return th.in.ok(&Record{Type: hm.Types["Response"], Fields: []Value{int64(resp.StatusCode), headers, string(body)}})
}

func (th *Thread) serve(port int64, handler Value) Value {
	hm := th.httpModule()
	reqType := hm.Types["Request"]
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		query := NewMap()
		for k, vs := range r.URL.Query() {
			query = query.Put(k, strings.Join(vs, ","))
		}
		headers := NewMap()
		for k, vs := range r.Header {
			headers = headers.Put(strings.ToLower(k), strings.Join(vs, ", "))
		}
		req := &Record{Type: reqType, Fields: []Value{r.Method, r.URL.Path, query, headers, string(body)}}
		t := th.in.NewThread(th.mod)
		t.stack = append([]stackEntry(nil), th.stack...)
		var resp *Record
		err := t.Protect(func() { resp = t.callValue(handler, []Value{req}, th.site()).(*Record) })
		if err != nil {
			th.in.errOut("http handler error: " + err.Error() + "\n")
			http.Error(w, "internal server error", 500)
			return
		}
		resp.Fields[1].(Map).Range(func(k, v Value) bool {
			w.Header().Set(k.(string), v.(string))
			return true
		})
		w.WriteHeader(int(resp.Fields[0].(int64)))
		io.WriteString(w, resp.Fields[2].(string))
	})
	addr := fmt.Sprintf(":%d", port)
	th.in.errOut(fmt.Sprintf("listening on http://localhost%s\n", addr))
	if err := http.ListenAndServe(addr, h); err != nil {
		return th.in.err(err.Error())
	}
	return th.in.ok(Unit)
}
