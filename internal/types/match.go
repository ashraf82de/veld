package types

import (
	"strconv"
	"strings"

	"github.com/ashraf82de/veld/internal/diag"
	"github.com/ashraf82de/veld/internal/syntax"
)

func (c *Checker) checkMatch(e *syntax.MatchExpr, want Type) Type {
	st := c.checkExpr(e.X, nil)
	var rows []spat
	for i, arm := range e.Arms {
		c.pushScope()
		bound := map[string]bool{}
		c.checkPattern(arm.Pattern, st, bound)
		if arm.Guard != nil {
			c.expectType(arm.Guard, TBool, "match guard")
		}
		if want == nil {
			c.checkBlock(arm.Body, nil)
		} else {
			c.checkBlock(arm.Body, want)
		}
		c.popScope()
		reachable := false
		sps := c.toSpats(arm.Pattern, st)
		for _, sp := range sps {
			if useful(rows, []Type{st}, []spat{sp}, c) {
				reachable = true
			}
		}
		if !reachable {
			c.diags.Warnf("W501", arm.Span, "this case is unreachable: earlier cases already match every value it could match")
		}
		_ = i
		if arm.Guard == nil {
			rows = append(rows, sps...)
		}
	}
	if isUnresolved(st) {
		return retOrUnit(want)
	}
	if w := missing(toMatrix(rows), []Type{st}, c); w != nil {
		ex := showSpat(w[0])
		if e.Values != nil {
			// Show `A, B` rather than the TupleN constructor the parser used.
			ex = strings.TrimSuffix(strings.TrimPrefix(ex, "Tuple"+strconv.Itoa(len(e.Values))+"("), ")")
		}
		d := c.errf("E501", e.Span, "match is not exhaustive: missing case `%s`", ex)
		indent := strings.Repeat(" ", max(0, e.EndPos.Col-1)+2)
		d.WithFix("add the missing case", diag.Span{File: e.Span.File, Start: lineStart(e.EndPos), End: lineStart(e.EndPos)}, indent+"case "+ex+" => ???\n")
		if hasGuards(e) {
			d.Note("cases with `if` guards do not count towards exhaustiveness")
		}
	}
	return retOrUnit(want)
}

func lineStart(p diag.Pos) diag.Pos {
	return diag.Pos{Line: p.Line, Col: 1, Offset: p.Offset - (p.Col - 1)}
}

func retOrUnit(want Type) Type {
	if want == nil {
		return TUnit
	}
	return want
}

func hasGuards(e *syntax.MatchExpr) bool {
	for _, a := range e.Arms {
		if a.Guard != nil {
			return true
		}
	}
	return false
}

func (c *Checker) checkPattern(p syntax.Pattern, t Type, bound map[string]bool) {
	switch p := p.(type) {
	case *syntax.WildPat:
	case *syntax.OrPat:
		for _, a := range p.Alts {
			inner := map[string]bool{}
			c.pushScope()
			c.checkPattern(a, t, inner)
			for _, l := range c.sc.order {
				l.used = true
			}
			c.sc = c.sc.parent
			if len(inner) > 0 {
				c.errf("E504", a.Sp(), "alternatives in an or-pattern (`a | b`) cannot bind variables").
					Note("use separate cases, or `_` for the fields")
			}
		}
	case *syntax.BindPat:
		if bound[p.Name] {
			c.errf("E502", p.Span, "`%s` is bound twice in the same pattern", p.Name)
		}
		bound[p.Name] = true
		c.declare(p.Name, t, false, p.Span, diag.Span{}, false)
	case *syntax.LitPat:
		var lt Type
		switch p.Value.(type) {
		case *syntax.IntLit:
			lt = TInt
		case *syntax.StrLit:
			lt = TStr
		case *syntax.BoolLit:
			lt = TBool
		}
		c.unifyAt(t, lt, p.Span, "pattern")
	case *syntax.ListPat:
		elem := c.fresh()
		c.unifyAt(t, ListOf(elem), p.Span, "list pattern")
		for _, e := range p.Elems {
			c.checkPattern(e, elem, bound)
		}
		for _, e := range p.Suffix {
			c.checkPattern(e, elem, bound)
		}
		if p.HasRest && p.Rest != "" && p.Rest != "_" {
			if bound[p.Rest] {
				c.errf("E502", p.Span, "`%s` is bound twice in the same pattern", p.Rest)
			}
			bound[p.Rest] = true
			c.declare(p.Rest, ListOf(elem), false, p.Span, diag.Span{}, false)
		}
	case *syntax.CtorPat:
		ci := c.lookupCtor(p.Module, p.Name, p.Span)
		if ci == nil {
			for _, a := range p.Args {
				c.checkPattern(a, c.fresh(), bound)
			}
			return
		}
		ct, m := c.instantiateType(ci.Type)
		if !Unify(t, ct) {
			c.errf("E301", p.Span, "pattern `%s` belongs to type %s, but the value has type %s", p.Name, ci.Type.Name, Show(t))
		}
		if len(p.Args) != len(ci.Fields) {
			if len(ci.Fields) > 0 && !p.HasParens {
				under := strings.TrimSuffix(strings.Repeat("_, ", len(ci.Fields)), ", ")
				c.errf("E503", p.Span, "constructor `%s` has %d field(s); match them: `%s(%s)`", p.Name, len(ci.Fields), p.Name, under).
					WithFix("add field patterns", diag.Span{File: p.Span.File, Start: p.Span.End, End: p.Span.End}, "("+under+")")
			} else {
				c.errf("E503", p.Span, "constructor `%s` has %d field(s), but the pattern has %d", p.Name, len(ci.Fields), len(p.Args))
			}
		}
		for i, a := range p.Args {
			if i < len(ci.Fields) {
				c.checkPattern(a, Subst(ci.Fields[i].Type, m), bound)
			} else {
				c.checkPattern(a, c.fresh(), bound)
			}
		}
	case *syntax.RecordPat:
		c.checkRecordPattern(p, t, bound)
	}
}

// findRecord resolves the record type a pattern names, without reporting.
func (c *Checker) findRecord(module, name string) *TypeInfo {
	var ti *TypeInfo
	if module != "" {
		if dep, ok := c.mod.Imports[module]; ok {
			ti = dep.Types[name]
		}
	} else {
		ti = c.mod.Types[name]
		if ti == nil && c.prelude != nil {
			ti = c.prelude.Types[name]
		}
	}
	if ti != nil && ti.IsRecord {
		return ti
	}
	return nil
}

func (c *Checker) checkRecordPattern(p *syntax.RecordPat, t Type, bound map[string]bool) {
	ti := c.findRecord(p.Module, p.Name)
	if ti == nil {
		what := "unknown record type `" + p.Name + "`"
		if p.Module != "" {
			if _, ok := c.mod.Imports[p.Module]; !ok {
				c.unknownModule(p.Module, p.Span)
				what = ""
			}
		} else if other := c.mod.Types[p.Name]; other != nil {
			what = "`" + p.Name + "` is a sum type, not a record; match its variants, e.g. `" + firstCtor(other) + "(...)`"
		}
		if what != "" {
			d := c.errf("E511", p.Span, "%s", what)
			if s := diag.Suggest(p.Name, keys(c.mod.Types)); s != "" && p.Module == "" {
				d.Note("did you mean `%s`?", s)
			}
		}
		for _, fp := range p.Fields {
			c.checkPattern(fp.Pat, c.fresh(), bound)
		}
		return
	}
	if p.Module != "" && !ti.Pub {
		c.errf("E206", p.Span, "record `%s.%s` is not `pub`", p.Module, p.Name)
	}
	ct, m := c.instantiateType(ti)
	if !Unify(t, ct) {
		c.errf("E301", p.Span, "pattern `%s{...}` matches type %s, but the value has type %s", p.Name, ti.Name, Show(t))
	}
	seen := map[string]bool{}
	for _, fp := range p.Fields {
		fd, _, ok := ti.Field(fp.Name)
		if !ok {
			d := c.errf("E511", fp.Span, "record `%s` has no field `%s`", ti.Name, fp.Name)
			var names []string
			for _, x := range ti.Fields {
				names = append(names, x.Name)
			}
			if s := diag.Suggest(fp.Name, names); s != "" && fp.Shorthand {
				d.Note("did you mean `%s`?", s)
			}
			c.checkPattern(fp.Pat, c.fresh(), bound)
			continue
		}
		if seen[fp.Name] {
			c.errf("E502", fp.Span, "field `%s` is matched twice in the same pattern", fp.Name)
		}
		seen[fp.Name] = true
		c.checkPattern(fp.Pat, Subst(fd.Type, m), bound)
	}
	if !p.HasRest && len(seen) < len(ti.Fields) {
		var missing []string
		for _, f := range ti.Fields {
			if !seen[f.Name] {
				missing = append(missing, f.Name)
			}
		}
		d := c.errf("E510", p.Span, "record pattern `%s{...}` does not mention field(s) %s; list them or end the pattern with `..`", ti.Name, strings.Join(missing, ", "))
		at := diag.Pos{Line: p.Span.End.Line, Col: p.Span.End.Col - 1, Offset: p.Span.End.Offset - 1}
		text := ", .."
		if len(p.Fields) == 0 {
			text = ".."
		}
		d.WithFix("ignore the other fields with `..`", diag.Span{File: p.Span.File, Start: at, End: at}, text)
	}
}

// recordCtor is the name of the single constructor of a record in the
// exhaustiveness analysis; it encodes the field names for witnesses.
func recordCtor(ti *TypeInfo) string {
	names := make([]string, len(ti.Fields))
	for i, f := range ti.Fields {
		names[i] = f.Name
	}
	return "{" + ti.Name + "|" + strings.Join(names, ",") + "}"
}

// ---------- exhaustiveness (Maranget-style usefulness) ----------

// spat is a simplified pattern: a wildcard (ctor == "") or a constructor
// applied to sub-patterns. Lists are encoded as ::(head, tail) / [].
type spat struct {
	ctor string
	args []spat
}

var wild = spat{}

// toSpats converts a pattern to simplified patterns, expanding or-patterns
// into one alternative each.
func (c *Checker) toSpats(p syntax.Pattern, t Type) []spat {
	switch p := p.(type) {
	case *syntax.WildPat, *syntax.BindPat:
		return []spat{wild}
	case *syntax.OrPat:
		var out []spat
		for _, a := range p.Alts {
			out = append(out, c.toSpats(a, t)...)
		}
		return out
	case *syntax.LitPat:
		switch v := p.Value.(type) {
		case *syntax.BoolLit:
			return []spat{{ctor: strconv.FormatBool(v.Value)}}
		case *syntax.IntLit:
			return []spat{{ctor: "#" + strconv.FormatInt(v.Value, 10)}}
		case *syntax.StrLit:
			s := ""
			for _, seg := range v.Segs {
				s += seg.Lit
			}
			return []spat{{ctor: "$" + strconv.Quote(s)}}
		}
	case *syntax.CtorPat:
		arity := len(p.Args)
		if ci := c.ctorByName(t, p.Name); ci != nil {
			arity = len(ci.Fields)
		}
		ats := argTypes(t, p.Name, arity, c)
		args := make([][]spat, arity)
		for i := 0; i < arity; i++ {
			if i < len(p.Args) {
				args[i] = c.toSpats(p.Args[i], ats[i])
			} else {
				args[i] = []spat{wild}
			}
		}
		var out []spat
		for _, combo := range product(args) {
			out = append(out, spat{ctor: p.Name, args: combo})
		}
		return out
	case *syntax.RecordPat:
		ti := c.findRecord(p.Module, p.Name)
		if ti == nil {
			return []spat{wild}
		}
		ctor := recordCtor(ti)
		ats := argTypes(t, ctor, len(ti.Fields), c)
		args := make([][]spat, len(ti.Fields))
		for i, f := range ti.Fields {
			args[i] = []spat{wild}
			for _, fp := range p.Fields {
				if fp.Name == f.Name {
					args[i] = c.toSpats(fp.Pat, ats[i])
				}
			}
		}
		var out []spat
		for _, combo := range product(args) {
			out = append(out, spat{ctor: ctor, args: combo})
		}
		return out
	case *syntax.ListPat:
		var elem Type = c.fresh()
		if tc, ok := Prune(t).(*TCon); ok && tc.Info == nil && tc.Name == "List" {
			elem = tc.Args[0]
		}
		tails := []spat{{ctor: "[]"}}
		if p.HasRest {
			tails = []spat{wild}
		}
		if len(p.Suffix) > 0 {
			// Patterns with elements after `..rest` are not analysed
			// precisely: they are treated as covering nothing, so matches
			// using them need a catch-all case.
			c.suffixID++
			tails = []spat{{ctor: "?suffix" + strconv.Itoa(c.suffixID)}}
		}
		for i := len(p.Elems) - 1; i >= 0; i-- {
			var next []spat
			for _, h := range c.toSpats(p.Elems[i], elem) {
				for _, tl := range tails {
					next = append(next, spat{ctor: "::", args: []spat{h, tl}})
				}
			}
			tails = next
		}
		return tails
	}
	return []spat{wild}
}

// product returns the cartesian product of the alternative lists.
func product(lists [][]spat) [][]spat {
	out := [][]spat{{}}
	for _, l := range lists {
		var next [][]spat
		for _, prefix := range out {
			for _, x := range l {
				next = append(next, append(append([]spat{}, prefix...), x))
			}
		}
		out = next
	}
	return out
}

func (c *Checker) ctorByName(t Type, name string) *CtorInfo {
	if tc, ok := Prune(t).(*TCon); ok && tc.Info != nil {
		for _, ci := range tc.Info.Ctors {
			if ci.Name == name {
				return ci
			}
		}
	}
	return nil
}

type ctorSig struct {
	name  string
	arity int
}

// signature returns the complete constructor list for t, or nil if t has an
// unbounded set of values (Int, Str, ...).
func signature(t Type) []ctorSig {
	tc, ok := Prune(t).(*TCon)
	if !ok {
		return nil
	}
	if tc.Info != nil && !tc.Info.IsRecord {
		var out []ctorSig
		for _, ci := range tc.Info.Ctors {
			out = append(out, ctorSig{ci.Name, len(ci.Fields)})
		}
		return out
	}
	if tc.Info != nil && tc.Info.IsRecord {
		return []ctorSig{{recordCtor(tc.Info), len(tc.Info.Fields)}}
	}
	switch tc.Name {
	case "Bool":
		return []ctorSig{{"true", 0}, {"false", 0}}
	case "List":
		return []ctorSig{{"[]", 0}, {"::", 2}}
	}
	return nil
}

// argTypes returns the types of constructor ctor's fields when applied at t.
func argTypes(t Type, ctor string, arity int, c *Checker) []Type {
	tc, _ := Prune(t).(*TCon)
	if tc != nil && tc.Info == nil && tc.Name == "List" && ctor == "::" {
		return []Type{tc.Args[0], t}
	}
	if tc != nil && tc.Info != nil && tc.Info.IsRecord && ctor == recordCtor(tc.Info) {
		m := map[int]Type{}
		for i, g := range tc.Info.TParams {
			m[g.ID] = tc.Args[i]
		}
		out := make([]Type, len(tc.Info.Fields))
		for i, f := range tc.Info.Fields {
			out[i] = Subst(f.Type, m)
		}
		return out
	}
	if tc != nil && tc.Info != nil {
		for _, ci := range tc.Info.Ctors {
			if ci.Name == ctor {
				m := map[int]Type{}
				for i, g := range tc.Info.TParams {
					m[g.ID] = tc.Args[i]
				}
				out := make([]Type, len(ci.Fields))
				for i, f := range ci.Fields {
					out[i] = Subst(f.Type, m)
				}
				return out
			}
		}
	}
	out := make([]Type, arity)
	for i := range out {
		out[i] = c.fresh()
	}
	return out
}

func toMatrix(rows []spat) [][]spat {
	out := make([][]spat, len(rows))
	for i, r := range rows {
		out[i] = []spat{r}
	}
	return out
}

func specialize(m [][]spat, ctor string, arity int) [][]spat {
	var out [][]spat
	for _, row := range m {
		h := row[0]
		if h.ctor == "" {
			nr := make([]spat, 0, arity+len(row)-1)
			for i := 0; i < arity; i++ {
				nr = append(nr, wild)
			}
			out = append(out, append(nr, row[1:]...))
		} else if h.ctor == ctor {
			nr := append(append([]spat{}, h.args...), row[1:]...)
			out = append(out, nr)
		}
	}
	return out
}

func defaultMatrix(m [][]spat) [][]spat {
	var out [][]spat
	for _, row := range m {
		if row[0].ctor == "" {
			out = append(out, row[1:])
		}
	}
	return out
}

func headCtors(m [][]spat) map[string]int {
	out := map[string]int{}
	for _, row := range m {
		if row[0].ctor != "" {
			out[row[0].ctor] = len(row[0].args)
		}
	}
	return out
}

func useful(rows []spat, types []Type, vec []spat, c *Checker) bool {
	return usefulM(toMatrix(rows), types, vec, c)
}

func usefulM(m [][]spat, types []Type, vec []spat, c *Checker) bool {
	if len(vec) == 0 {
		return len(m) == 0
	}
	t := types[0]
	h := vec[0]
	if h.ctor != "" {
		ats := argTypes(t, h.ctor, len(h.args), c)
		return usefulM(specialize(m, h.ctor, len(h.args)), append(ats, types[1:]...), append(append([]spat{}, h.args...), vec[1:]...), c)
	}
	sig := signature(t)
	heads := headCtors(m)
	if sig != nil && complete(sig, heads) {
		for _, cs := range sig {
			ats := argTypes(t, cs.name, cs.arity, c)
			nv := make([]spat, cs.arity)
			if usefulM(specialize(m, cs.name, cs.arity), append(ats, types[1:]...), append(nv, vec[1:]...), c) {
				return true
			}
		}
		return false
	}
	return usefulM(defaultMatrix(m), types[1:], vec[1:], c)
}

func complete(sig []ctorSig, heads map[string]int) bool {
	for _, s := range sig {
		if _, ok := heads[s.name]; !ok {
			return false
		}
	}
	return true
}

// missing returns a witness vector of values not matched by m, or nil.
func missing(m [][]spat, types []Type, c *Checker) []spat {
	if len(types) == 0 {
		if len(m) == 0 {
			return []spat{}
		}
		return nil
	}
	t := types[0]
	sig := signature(t)
	heads := headCtors(m)
	if sig != nil && complete(sig, heads) {
		for _, cs := range sig {
			ats := argTypes(t, cs.name, cs.arity, c)
			w := missing(specialize(m, cs.name, cs.arity), append(ats, types[1:]...), c)
			if w != nil {
				head := spat{ctor: cs.name, args: append([]spat{}, w[:cs.arity]...)}
				return append([]spat{head}, w[cs.arity:]...)
			}
		}
		return nil
	}
	w := missing(defaultMatrix(m), types[1:], c)
	if w == nil {
		return nil
	}
	if sig != nil && len(heads) > 0 {
		for _, cs := range sig {
			if _, ok := heads[cs.name]; !ok {
				args := make([]spat, cs.arity)
				return append([]spat{{ctor: cs.name, args: args}}, w...)
			}
		}
	}
	return append([]spat{wild}, w...)
}

func showSpat(p spat) string {
	switch {
	case strings.HasPrefix(p.ctor, "?suffix"):
		return ".._"
	case p.ctor == "":
		return "_"
	case p.ctor == "[]":
		return "[]"
	case p.ctor == "::":
		var elems []string
		cur := p
		for cur.ctor == "::" {
			elems = append(elems, showSpat(cur.args[0]))
			cur = cur.args[1]
		}
		if cur.ctor == "" {
			elems = append(elems, ".._")
		}
		return "[" + strings.Join(elems, ", ") + "]"
	case p.ctor[0] == '{':
		name, fields, _ := strings.Cut(strings.Trim(p.ctor, "{}"), "|")
		names := strings.Split(fields, ",")
		parts := make([]string, len(p.args))
		for i, a := range p.args {
			if i < len(names) {
				parts[i] = names[i] + ": " + showSpat(a)
			}
		}
		return name + "{" + strings.Join(parts, ", ") + "}"
	case p.ctor[0] == '#':
		return p.ctor[1:]
	case p.ctor[0] == '$':
		return p.ctor[1:]
	}
	if len(p.args) == 0 {
		return p.ctor
	}
	parts := make([]string, len(p.args))
	for i, a := range p.args {
		parts[i] = showSpat(a)
	}
	return p.ctor + "(" + strings.Join(parts, ", ") + ")"
}
