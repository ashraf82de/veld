package interp

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"
)

// stdRecord builds a record of a type declared in a standard library module.
func (th *Thread) stdRecord(module, name string, fields ...Value) Value {
	return &Record{Type: th.in.Prog.ByPath[module].Types[name], Fields: fields}
}

// pair builds a prelude Pair.
func (th *Thread) pair(a, b Value) Value {
	return &Record{Type: th.in.Prog.Prelude.Types["Pair"], Fields: []Value{a, b}}
}

var regexCache sync.Map // pattern -> *regexp.Regexp

func compileRegex(pattern string) (*regexp.Regexp, error) {
	if re, ok := regexCache.Load(pattern); ok {
		return re.(*regexp.Regexp), nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	regexCache.Store(pattern, re)
	return re, nil
}

// store is the process-wide state behind std.state.
type store struct {
	mu    sync.Mutex
	vals  map[string]storeEntry
	order []string
	gen   uint64
}

type storeEntry struct {
	val Value
	gen uint64
}

func (s *store) init() {
	if s.vals == nil {
		s.vals = map[string]storeEntry{}
	}
}

func (s *store) put(key string, v Value) {
	s.init()
	if _, ok := s.vals[key]; !ok {
		s.order = append(s.order, key)
	}
	s.gen++
	s.vals[key] = storeEntry{val: v, gen: s.gen}
}

func (s *store) del(key string) {
	s.init()
	if _, ok := s.vals[key]; !ok {
		return
	}
	delete(s.vals, key)
	for i, k := range s.order {
		if k == key {
			s.order = append(s.order[:i:i], s.order[i+1:]...)
			break
		}
	}
	s.gen++
}

var globalStore store

func registerExt(def func(name string, f func(th *Thread, a []Value) Value)) {
	// ---- str ----
	def("str.at", func(th *Thread, a []Value) Value {
		s, i := a[0].(string), a[1].(int64)
		if i < 0 {
			return th.in.none()
		}
		if int64(len(s)) == int64(utf8.RuneCountInString(s)) {
			if i >= int64(len(s)) {
				return th.in.none()
			}
			return th.in.some(s[i : i+1])
		}
		n := int64(0)
		for _, r := range s {
			if n == i {
				return th.in.some(string(r))
			}
			n++
		}
		return th.in.none()
	})
	class := func(name string, ok func(r rune) bool) {
		def(name, func(th *Thread, a []Value) Value {
			s := a[0].(string)
			if s == "" {
				return false
			}
			for _, r := range s {
				if !ok(r) {
					return false
				}
			}
			return true
		})
	}
	class("str.is_digit", unicode.IsDigit)
	class("str.is_alpha", unicode.IsLetter)
	class("str.is_alnum", func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) })
	class("str.is_space", unicode.IsSpace)
	class("str.is_upper", unicode.IsUpper)
	class("str.is_lower", unicode.IsLower)
	def("str.strip_prefix", func(th *Thread, a []Value) Value {
		if r, ok := strings.CutPrefix(a[0].(string), a[1].(string)); ok {
			return th.in.some(r)
		}
		return th.in.none()
	})
	def("str.strip_suffix", func(th *Thread, a []Value) Value {
		if r, ok := strings.CutSuffix(a[0].(string), a[1].(string)); ok {
			return th.in.some(r)
		}
		return th.in.none()
	})
	def("str.split_once", func(th *Thread, a []Value) Value {
		if a[1].(string) == "" {
			th.nativeFail("str.split_once: separator must not be empty")
		}
		if before, after, ok := strings.Cut(a[0].(string), a[1].(string)); ok {
			return th.in.some(th.pair(before, after))
		}
		return th.in.none()
	})

	// ---- list ----
	def("list.zip", func(th *Thread, a []Value) Value {
		x, y := a[0].(List), a[1].(List)
		n := min(x.Len(), y.Len())
		out := make([]Value, n)
		for i := 0; i < n; i++ {
			out[i] = th.pair(x.Get(i), y.Get(i))
		}
		return NewList(out)
	})
	def("list.enumerate", func(th *Thread, a []Value) Value {
		l := a[0].(List)
		out := make([]Value, 0, l.Len())
		l.Range(func(i int, x Value) bool {
			out = append(out, th.pair(int64(i), x))
			return true
		})
		return NewList(out)
	})
	def("list.group_by", func(th *Thread, a []Value) Value {
		groups := NewMap()
		a[0].(List).Range(func(_ int, x Value) bool {
			k := th.callValue(a[1], []Value{x}, th.site())
			cur, ok := groups.Get(k)
			if !ok {
				cur = EmptyList()
			}
			groups = groups.Put(k, cur.(List).Push(x))
			return true
		})
		return groups
	})
	extremeBy := func(sign int) func(th *Thread, a []Value) Value {
		return func(th *Thread, a []Value) Value {
			l := a[0].(List)
			if l.Len() == 0 {
				return th.in.none()
			}
			var best, bestKey Value
			l.Range(func(i int, x Value) bool {
				k := th.callValue(a[1], []Value{x}, th.site())
				if i == 0 || Compare(k, bestKey)*sign > 0 {
					best, bestKey = x, k
				}
				return true
			})
			return th.in.some(best)
		}
	}
	def("list.max_by", extremeBy(1))
	def("list.min_by", extremeBy(-1))
	def("list.sort_desc", func(th *Thread, a []Value) Value { return sortValues(a[0].(List), nil, true) })
	def("list.sort_by_desc", func(th *Thread, a []Value) Value {
		return sortValues(a[0].(List), func(v Value) Value { return th.callValue(a[1], []Value{v}, th.site()) }, true)
	})

	// ---- map ----
	def("map.entries", func(th *Thread, a []Value) Value {
		m := a[0].(Map)
		out := make([]Value, 0, m.Len())
		m.Range(func(k, v Value) bool {
			out = append(out, th.pair(k, v))
			return true
		})
		return NewList(out)
	})

	// ---- math ----
	f1 := func(name string, f func(float64) float64) {
		def(name, func(th *Thread, a []Value) Value { return f(a[0].(float64)) })
	}
	f1("math.tan", math.Tan)
	f1("math.asin", math.Asin)
	f1("math.acos", math.Acos)
	f1("math.atan", math.Atan)
	f1("math.log10", math.Log10)
	f1("math.trunc", math.Trunc)
	def("math.atan2", func(th *Thread, a []Value) Value { return math.Atan2(a[0].(float64), a[1].(float64)) })
	def("math.is_nan", func(th *Thread, a []Value) Value { return math.IsNaN(a[0].(float64)) })

	// ---- regex ----
	withRe := func(name string, f func(th *Thread, re *regexp.Regexp, a []Value) Value) {
		def("regex."+name, func(th *Thread, a []Value) Value {
			pattern := a[0].(string)
			if name == "replace" || name == "split" {
				pattern = a[1].(string)
			}
			re, err := compileRegex(pattern)
			if err != nil {
				return th.in.err("invalid regex: " + err.Error())
			}
			return th.in.ok(f(th, re, a))
		})
	}
	withRe("is_match", func(th *Thread, re *regexp.Regexp, a []Value) Value { return re.MatchString(a[1].(string)) })
	withRe("find", func(th *Thread, re *regexp.Regexp, a []Value) Value {
		s := a[1].(string)
		if loc := re.FindStringIndex(s); loc != nil {
			return th.in.some(s[loc[0]:loc[1]])
		}
		return th.in.none()
	})
	withRe("find_all", func(th *Thread, re *regexp.Regexp, a []Value) Value {
		return strList(re.FindAllString(a[1].(string), -1))
	})
	withRe("captures", func(th *Thread, re *regexp.Regexp, a []Value) Value {
		if m := re.FindStringSubmatch(a[1].(string)); m != nil {
			return th.in.some(strList(m))
		}
		return th.in.none()
	})
	withRe("replace", func(th *Thread, re *regexp.Regexp, a []Value) Value {
		return re.ReplaceAllString(a[0].(string), a[2].(string))
	})
	withRe("split", func(th *Thread, re *regexp.Regexp, a []Value) Value {
		return strList(re.Split(a[0].(string), -1))
	})

	// ---- path ----
	def("path.join", func(th *Thread, a []Value) Value {
		var parts []string
		a[0].(List).Range(func(_ int, x Value) bool {
			parts = append(parts, filepath.ToSlash(x.(string)))
			return true
		})
		return path.Join(parts...)
	})
	def("path.base", func(th *Thread, a []Value) Value { return path.Base(filepath.ToSlash(a[0].(string))) })
	def("path.dir", func(th *Thread, a []Value) Value { return path.Dir(filepath.ToSlash(a[0].(string))) })
	def("path.ext", func(th *Thread, a []Value) Value { return path.Ext(filepath.ToSlash(a[0].(string))) })
	def("path.clean", func(th *Thread, a []Value) Value { return path.Clean(filepath.ToSlash(a[0].(string))) })
	def("path.is_absolute", func(th *Thread, a []Value) Value { return path.IsAbs(filepath.ToSlash(a[0].(string))) })

	// ---- encoding ----
	def("encoding.base64_encode", func(th *Thread, a []Value) Value {
		return base64.StdEncoding.EncodeToString([]byte(a[0].(string)))
	})
	def("encoding.base64_decode", func(th *Thread, a []Value) Value {
		b, err := base64.StdEncoding.DecodeString(a[0].(string))
		if err != nil {
			return th.in.err("invalid base64: " + err.Error())
		}
		if !utf8.Valid(b) {
			return th.in.err("decoded bytes are not valid UTF-8 text")
		}
		return th.in.ok(string(b))
	})
	def("encoding.hex_encode", func(th *Thread, a []Value) Value { return hex.EncodeToString([]byte(a[0].(string))) })
	def("encoding.hex_decode", func(th *Thread, a []Value) Value {
		b, err := hex.DecodeString(a[0].(string))
		if err != nil {
			return th.in.err("invalid hex: " + err.Error())
		}
		if !utf8.Valid(b) {
			return th.in.err("decoded bytes are not valid UTF-8 text")
		}
		return th.in.ok(string(b))
	})
	def("encoding.url_encode", func(th *Thread, a []Value) Value { return url.QueryEscape(a[0].(string)) })
	def("encoding.url_decode", func(th *Thread, a []Value) Value {
		s, err := url.QueryUnescape(a[0].(string))
		if err != nil {
			return th.in.err("invalid percent-encoding: " + err.Error())
		}
		return th.in.ok(s)
	})

	// ---- crypto ----
	def("crypto.sha256", func(th *Thread, a []Value) Value {
		h := sha256.Sum256([]byte(a[0].(string)))
		return hex.EncodeToString(h[:])
	})
	def("crypto.sha512", func(th *Thread, a []Value) Value {
		h := sha512.Sum512([]byte(a[0].(string)))
		return hex.EncodeToString(h[:])
	})
	def("crypto.hmac_sha256", func(th *Thread, a []Value) Value {
		m := hmac.New(sha256.New, []byte(a[0].(string)))
		m.Write([]byte(a[1].(string)))
		return hex.EncodeToString(m.Sum(nil))
	})
	def("crypto.random_hex", func(th *Thread, a []Value) Value {
		n := a[0].(int64)
		if n < 0 || n > 1<<20 {
			th.nativeFail("crypto.random_hex: bytes must be between 0 and %d, got %d", 1<<20, n)
		}
		b := make([]byte, n)
		if _, err := rand.Read(b); err != nil {
			th.nativeFail("crypto.random_hex: %v", err)
		}
		return hex.EncodeToString(b)
	})
	def("crypto.uuid", func(th *Thread, a []Value) Value {
		var b [16]byte
		if _, err := rand.Read(b[:]); err != nil {
			th.nativeFail("crypto.uuid: %v", err)
		}
		b[6] = b[6]&0x0f | 0x40
		b[8] = b[8]&0x3f | 0x80
		return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
	})
	def("crypto.constant_time_equal", func(th *Thread, a []Value) Value {
		return subtle.ConstantTimeCompare([]byte(a[0].(string)), []byte(a[1].(string))) == 1
	})

	// ---- csv ----
	def("csv.parse", func(th *Thread, a []Value) Value {
		r := csv.NewReader(strings.NewReader(a[0].(string)))
		r.FieldsPerRecord = -1
		recs, err := r.ReadAll()
		if err != nil {
			return th.in.err("invalid CSV: " + err.Error())
		}
		rows := make([]Value, len(recs))
		for i, rec := range recs {
			rows[i] = strList(rec)
		}
		return th.in.ok(NewList(rows))
	})
	def("csv.encode", func(th *Thread, a []Value) Value {
		var b bytes.Buffer
		w := csv.NewWriter(&b)
		a[0].(List).Range(func(_ int, row Value) bool {
			var rec []string
			row.(List).Range(func(_ int, f Value) bool {
				rec = append(rec, f.(string))
				return true
			})
			w.Write(rec)
			return true
		})
		w.Flush()
		return b.String()
	})

	// ---- process ----
	runProc := func(th *Thread, program string, args List, input *string) Value {
		var argv []string
		args.Range(func(_ int, x Value) bool { argv = append(argv, x.(string)); return true })
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, program, argv...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if input != nil {
			cmd.Stdin = strings.NewReader(*input)
		}
		err := cmd.Run()
		status := int64(0)
		if err != nil {
			ee, ok := err.(*exec.ExitError)
			if !ok {
				return th.in.err(err.Error())
			}
			status = int64(ee.ExitCode())
		}
		return th.in.ok(th.stdRecord("std.process", "Output", status, stdout.String(), stderr.String()))
	}
	def("process.run", func(th *Thread, a []Value) Value { return runProc(th, a[0].(string), a[1].(List), nil) })
	def("process.run_with_input", func(th *Thread, a []Value) Value {
		in := a[2].(string)
		return runProc(th, a[0].(string), a[1].(List), &in)
	})

	// ---- state ----
	def("state.get", func(th *Thread, a []Value) Value {
		globalStore.mu.Lock()
		defer globalStore.mu.Unlock()
		if e, ok := globalStore.vals[a[0].(string)]; ok {
			return th.in.some(e.val)
		}
		return th.in.none()
	})
	def("state.put", func(th *Thread, a []Value) Value {
		globalStore.mu.Lock()
		defer globalStore.mu.Unlock()
		globalStore.put(a[0].(string), a[1])
		return Unit
	})
	def("state.remove", func(th *Thread, a []Value) Value {
		globalStore.mu.Lock()
		defer globalStore.mu.Unlock()
		globalStore.del(a[0].(string))
		return Unit
	})
	def("state.keys", func(th *Thread, a []Value) Value {
		globalStore.mu.Lock()
		defer globalStore.mu.Unlock()
		return strList(append([]string(nil), globalStore.order...))
	})
	def("state.update", func(th *Thread, a []Value) Value {
		key := a[0].(string)
		for {
			globalStore.mu.Lock()
			e, present := globalStore.vals[key]
			globalStore.mu.Unlock()
			var old Value = th.in.none()
			if present {
				old = th.in.some(e.val)
			}
			// f runs outside the lock so that it may take as long as it needs
			// (and even use other state functions); the commit below retries
			// if someone else changed the key in the meantime.
			nv := th.callValue(a[1], []Value{old}, th.site())
			globalStore.mu.Lock()
			cur, nowPresent := globalStore.vals[key]
			if nowPresent == present && (!present || cur.gen == e.gen) {
				globalStore.put(key, nv)
				globalStore.mu.Unlock()
				return nv
			}
			globalStore.mu.Unlock()
		}
	})
	def("state.incr", func(th *Thread, a []Value) Value {
		key, by := a[0].(string), a[1].(int64)
		globalStore.mu.Lock()
		defer globalStore.mu.Unlock()
		cur := int64(0)
		if e, ok := globalStore.vals[key]; ok {
			v, isVariant := e.val.(*Variant)
			if !isVariant || v.Ctor.Name != "JNum" {
				th.nativeFail("state.incr: the value under %q is not a number", key)
			}
			cur = int64(v.Fields[0].(float64))
		}
		if (by > 0 && cur > math.MaxInt64-by) || (by < 0 && cur < math.MinInt64-by) {
			th.fail("R101", th.here(), "integer overflow in state.incr")
		}
		cur += by
		globalStore.put(key, th.in.ctor("JNum", float64(cur)))
		return cur
	})
	def("state.save", func(th *Thread, a []Value) Value {
		globalStore.mu.Lock()
		fields := NewMap()
		for _, k := range globalStore.order {
			fields = fields.Put(k, globalStore.vals[k].val)
		}
		globalStore.mu.Unlock()
		var b bytes.Buffer
		encodeJSON(&b, th.in.ctor("JObj", fields), "  ", "")
		b.WriteByte('\n')
		tmp := a[0].(string) + ".tmp"
		if err := os.WriteFile(tmp, b.Bytes(), 0o644); err != nil {
			return th.in.err(err.Error())
		}
		if err := os.Rename(tmp, a[0].(string)); err != nil {
			return th.in.err(err.Error())
		}
		return th.in.ok(Unit)
	})
	def("state.load", func(th *Thread, a []Value) Value {
		b, err := os.ReadFile(a[0].(string))
		if err != nil {
			return th.in.err(err.Error())
		}
		v, err := th.decodeJSON(jsonDecoder(string(b)))
		if err != nil {
			return th.in.err("invalid JSON: " + err.Error())
		}
		obj, ok := v.(*Variant)
		if !ok || obj.Ctor.Name != "JObj" {
			return th.in.err("state file must contain a JSON object")
		}
		globalStore.mu.Lock()
		defer globalStore.mu.Unlock()
		globalStore.vals, globalStore.order = map[string]storeEntry{}, nil
		obj.Fields[0].(Map).Range(func(k, val Value) bool {
			globalStore.put(k.(string), val)
			return true
		})
		return th.in.ok(Unit)
	})

	// ---- task ----
	def("task.parallel_map", func(th *Thread, a []Value) Value {
		items := a[0].(List).ToSlice()
		if capturesVar(a[1], 0) {
			th.nativeFail("task.parallel_map: the function captures a `var` variable; workers would race on it (pass values in with `let` or arguments)")
		}
		out := make([]Value, len(items))
		workers := min(runtime.GOMAXPROCS(0), len(items))
		var next int64 = -1
		var stop atomic.Bool
		var wg sync.WaitGroup
		type failure struct {
			index int
			val   any
		}
		var mu sync.Mutex
		var first *failure
		site := th.site()
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				wt := th.in.NewThread(th.mod)
				i := -1
				defer func() {
					if r := recover(); r != nil {
						stop.Store(true)
						mu.Lock()
						if first == nil || i < first.index {
							first = &failure{i, r}
						}
						mu.Unlock()
					}
				}()
				for !stop.Load() {
					i = int(atomic.AddInt64(&next, 1))
					if i >= len(items) {
						return
					}
					out[i] = wt.callValue(a[1], []Value{items[i]}, site)
				}
			}()
		}
		wg.Wait()
		if first != nil {
			panic(first.val)
		}
		return NewList(out)
	})

	// ---- datetime ----
	def("datetime.iso", func(th *Thread, a []Value) Value {
		return time.UnixMilli(a[0].(int64)).UTC().Format(time.RFC3339Nano)
	})
	def("datetime.parse_iso", func(th *Thread, a []Value) Value {
		t, err := time.Parse(time.RFC3339, a[0].(string))
		if err != nil {
			return th.in.none()
		}
		return th.in.some(t.UnixMilli())
	})
	def("datetime.parts", func(th *Thread, a []Value) Value {
		t := time.UnixMilli(a[0].(int64)).UTC()
		return th.stdRecord("std.datetime", "Parts", int64(t.Year()), int64(t.Month()), int64(t.Day()),
			int64(t.Hour()), int64(t.Minute()), int64(t.Second()))
	})
	def("datetime.from_parts", func(th *Thread, a []Value) Value {
		f := a[0].(*Record).Fields
		return time.Date(int(f[0].(int64)), time.Month(f[1].(int64)), int(f[2].(int64)), int(f[3].(int64)),
			int(f[4].(int64)), int(f[5].(int64)), 0, time.UTC).UnixMilli()
	})
	def("datetime.weekday", func(th *Thread, a []Value) Value {
		return int64(time.UnixMilli(a[0].(int64)).UTC().Weekday())
	})

	// ---- http ----
	def("http.request", func(th *Thread, a []Value) Value {
		req, err := http.NewRequest(a[0].(string), a[1].(string), strings.NewReader(a[3].(string)))
		if err != nil {
			return th.in.err(err.Error())
		}
		a[2].(Map).Range(func(k, v Value) bool {
			req.Header.Set(k.(string), v.(string))
			return true
		})
		resp, err := httpClient.Do(req)
		return th.httpResult(resp, err)
	})
}

func jsonDecoder(s string) *json.Decoder {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	return dec
}

// capturesVar reports whether a function value captured a mutable `var` (a
// *Box), directly or through a closure it captured.
func capturesVar(v Value, depth int) bool {
	c, ok := v.(*Closure)
	if !ok || depth > 8 {
		return false
	}
	for _, x := range c.caps {
		if _, isBox := x.(*Box); isBox || capturesVar(x, depth+1) {
			return true
		}
	}
	return false
}
