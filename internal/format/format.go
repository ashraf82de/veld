// Package format prints a Veld AST in the one canonical layout. Formatting
// is idempotent: format(format(x)) == format(x).
package format

import (
	"sort"
	"strings"

	"github.com/ashraf82de/veld/internal/syntax"
)

const maxWidth = 100

type printer struct {
	b      strings.Builder
	indent int
}

// File returns the canonical source for f.
func File(f *syntax.File) string {
	p := &printer{}
	uses := append([]*syntax.Use{}, f.Uses...)
	sort.SliceStable(uses, func(i, j int) bool {
		a, b := uses[i].Path, uses[j].Path
		as, bs := a[0] == "std", b[0] == "std"
		if as != bs {
			return as
		}
		return strings.Join(a, ".") < strings.Join(b, ".")
	})
	for _, u := range uses {
		p.comments(u.Leading)
		p.line("use " + strings.Join(u.Path, ".") + trail(u.Trailing))
	}
	for i, d := range f.Decls {
		if i > 0 || len(uses) > 0 {
			p.b.WriteString("\n")
		}
		p.decl(d)
	}
	if len(f.EndComments) > 0 {
		if len(f.Decls) > 0 || len(uses) > 0 {
			p.b.WriteString("\n")
		}
		p.comments(f.EndComments)
	}
	return p.b.String()
}

func trail(c string) string {
	if c == "" {
		return ""
	}
	return " " + c
}

func (p *printer) pad() string { return strings.Repeat("  ", p.indent) }

func (p *printer) line(s string) {
	p.b.WriteString(p.pad())
	p.b.WriteString(s)
	p.b.WriteString("\n")
}

func (p *printer) comments(cs []string) {
	for _, c := range cs {
		if c == "" {
			p.b.WriteString("\n")
		} else {
			p.line(c)
		}
	}
}

func (p *printer) docs(doc []string) {
	for _, d := range doc {
		if d == "" {
			p.line("##")
		} else {
			p.line("## " + d)
		}
	}
}

func (p *printer) decl(d syntax.Decl) {
	switch d := d.(type) {
	case *syntax.FnDecl:
		p.comments(d.Leading)
		p.docs(d.Doc)
		p.line(Signature(d) + trail(d.Trailing))
		if d.Extern {
			return
		}
		p.indent++
		for _, r := range d.Requires {
			p.line("requires " + p.expr(r))
		}
		for _, e := range d.Ensures {
			p.line("ensures " + p.expr(e))
		}
		p.stmts(d.Body)
		p.indent--
		p.line("end fn")
	case *syntax.TypeDecl:
		p.comments(d.Leading)
		p.docs(d.Doc)
		p.line(pubPrefix(d.Pub) + "type " + d.Name + tparams(d.TParams) + trail(d.Trailing))
		p.indent++
		for _, v := range d.Variants {
			p.comments(v.Leading)
			s := "case " + v.Name
			if len(v.Fields) > 0 {
				var fs []string
				for _, f := range v.Fields {
					fs = append(fs, f.Name+": "+Type(f.Type))
				}
				s += "(" + strings.Join(fs, ", ") + ")"
			}
			p.line(s + trail(v.Trailing))
		}
		p.comments(d.EndComments)
		p.indent--
		p.line("end type")
	case *syntax.RecordDecl:
		p.comments(d.Leading)
		p.docs(d.Doc)
		p.line(pubPrefix(d.Pub) + "record " + d.Name + tparams(d.TParams) + trail(d.Trailing))
		p.indent++
		for _, f := range d.Fields {
			p.comments(f.Leading)
			p.line(f.Name + ": " + Type(f.Type) + trail(f.Trailing))
		}
		p.comments(d.EndComments)
		p.indent--
		p.line("end record")
	case *syntax.TestDecl:
		p.comments(d.Leading)
		p.line("test " + quoteStr(d.Name) + trail(d.Trailing))
		p.indent++
		p.stmts(d.Body)
		p.indent--
		p.line("end test")
	}
}

func pubPrefix(pub bool) string {
	if pub {
		return "pub "
	}
	return ""
}

func tparams(ts []string) string {
	if len(ts) == 0 {
		return ""
	}
	return "[" + strings.Join(ts, ", ") + "]"
}

// Signature renders a function header.
func Signature(d *syntax.FnDecl) string {
	var b strings.Builder
	b.WriteString(pubPrefix(d.Pub))
	if d.Extern {
		b.WriteString("extern ")
	}
	b.WriteString("fn " + d.Name + tparams(d.TParams) + "(")
	for i, prm := range d.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(prm.Name + ": " + Type(prm.Type))
	}
	b.WriteString(")")
	if d.Ret != nil {
		b.WriteString(" -> " + Type(d.Ret))
	}
	if len(d.Effects) > 0 {
		b.WriteString(" uses " + strings.Join(d.Effects, ", "))
	}
	return b.String()
}

// Type renders a type expression.
func Type(t syntax.TypeExpr) string {
	switch t := t.(type) {
	case *syntax.NamedType:
		s := t.Name
		if t.Module != "" {
			s = t.Module + "." + s
		}
		if len(t.Args) > 0 {
			var as []string
			for _, a := range t.Args {
				as = append(as, Type(a))
			}
			s += "[" + strings.Join(as, ", ") + "]"
		}
		return s
	case *syntax.FnType:
		var ps []string
		for _, a := range t.Params {
			ps = append(ps, Type(a))
		}
		s := "Fn(" + strings.Join(ps, ", ") + ") -> " + Type(t.Ret)
		if len(t.Effects) > 0 {
			s += " uses " + strings.Join(t.Effects, ", ")
		}
		return s
	}
	return "_"
}

// ---- statements ----

func (p *printer) stmts(b *syntax.Block) {
	for i, s := range b.Stmts {
		m := s.M()
		if m.Blank && i > 0 {
			p.b.WriteString("\n")
		}
		p.comments(m.Leading)
		p.line(p.stmt(s) + trail(m.Trailing))
	}
	p.comments(b.EndComments)
}

func (p *printer) stmt(s syntax.Stmt) string {
	switch s := s.(type) {
	case *syntax.LetStmt:
		kw := "let "
		if s.Mutable {
			kw = "var "
		}
		ty := ""
		if s.Type != nil {
			ty = ": " + Type(s.Type)
		}
		return kw + s.Name + ty + " = " + p.expr(s.Value)
	case *syntax.SetStmt:
		return "set " + s.Name + " = " + p.expr(s.Value)
	case *syntax.ExpectStmt:
		return "expect " + p.expr(s.X)
	case *syntax.ExprStmt:
		return p.expr(s.X)
	}
	return ""
}

// ---- expressions ----

// sub renders a nested block-structured construct at one deeper indent and
// returns it without the trailing newline.
func (p *printer) sub(f func()) string {
	saved := p.b
	p.b = strings.Builder{}
	p.indent++
	f()
	p.indent--
	out := p.b.String()
	p.b = saved
	return out
}

func (p *printer) expr(e syntax.Expr) string {
	switch e := e.(type) {
	case *syntax.IntLit:
		return e.Text
	case *syntax.FloatLit:
		return e.Text
	case *syntax.BoolLit:
		if e.Value {
			return "true"
		}
		return "false"
	case *syntax.StrLit:
		return p.str(e)
	case *syntax.Ident:
		return e.Name
	case *syntax.FieldExpr:
		return p.expr(e.X) + "." + e.Name
	case *syntax.ParenExpr:
		return "(" + p.expr(e.X) + ")"
	case *syntax.UnaryExpr:
		if e.Op == syntax.KW_NOT {
			return "not " + p.expr(e.X)
		}
		return "-" + p.expr(e.X)
	case *syntax.BinaryExpr:
		return p.expr(e.L) + " " + e.Op.String() + " " + p.expr(e.R)
	case *syntax.PipeExpr:
		// Flatten the chain; two or more stages go one per line.
		var stages []syntax.Expr
		var head syntax.Expr = e
		for {
			pe, ok := head.(*syntax.PipeExpr)
			if !ok {
				break
			}
			stages = append([]syntax.Expr{pe.R}, stages...)
			head = pe.L
		}
		parts := []string{p.expr(head)}
		for _, st := range stages {
			parts = append(parts, p.expr(st))
		}
		one := strings.Join(parts, " |> ")
		if len(stages) < 2 && len(p.pad())+len(one) <= maxWidth && !strings.Contains(one, "\n") {
			return one
		}
		var b strings.Builder
		b.WriteString(parts[0])
		for _, part := range parts[1:] {
			b.WriteString("\n" + p.pad() + "  |> " + strings.ReplaceAll(part, "\n", "\n  "))
		}
		return b.String()
	case *syntax.TryExpr:
		if pe, ok := e.X.(*syntax.PipeExpr); ok {
			// Printed as `x |> f()?`, which parses back to the same tree.
			return p.expr(pe) + "?"
		}
		return p.expr(e.X) + "?"
	case *syntax.CallExpr:
		var items []string
		for _, a := range e.Args {
			if a.Name != "" {
				items = append(items, a.Name+": "+p.expr(a.Value))
			} else {
				items = append(items, p.expr(a.Value))
			}
		}
		return p.expr(e.Fn) + p.list("(", ")", items)
	case *syntax.ListLit:
		var items []string
		for _, x := range e.Elems {
			items = append(items, p.expr(x))
		}
		return p.list("[", "]", items)
	case *syntax.MapLit:
		var items []string
		for _, en := range e.Entries {
			items = append(items, p.expr(en.Key)+": "+p.expr(en.Value))
		}
		return p.list("{", "}", items)
	case *syntax.RecordLit:
		var items []string
		if e.Base != nil {
			items = append(items, ".."+p.expr(e.Base))
		}
		for _, f := range e.Fields {
			items = append(items, f.Name+": "+p.expr(f.Value))
		}
		name := e.Name
		if e.Module != "" {
			name = e.Module + "." + name
		}
		return name + p.list("{", "}", items)
	case *syntax.LambdaExpr:
		var ps []string
		for _, prm := range e.Params {
			if prm.Type != nil {
				ps = append(ps, prm.Name+": "+Type(prm.Type))
			} else {
				ps = append(ps, prm.Name)
			}
		}
		head := "fn(" + strings.Join(ps, ", ") + ")"
		if e.Ret != nil {
			head += " -> " + Type(e.Ret)
		}
		if x := simpleBody(e.Body); x != nil {
			return head + " => " + p.expr(x)
		}
		return head + "\n" + p.sub(func() { p.stmts(e.Body) }) + p.pad() + "end fn"
	case *syntax.IfExpr:
		var b strings.Builder
		for i, br := range e.Branches {
			if i == 0 {
				b.WriteString("if " + p.expr(br.Cond) + "\n")
			} else {
				b.WriteString(p.pad() + "else if " + p.expr(br.Cond) + "\n")
			}
			b.WriteString(p.sub(func() { p.stmts(br.Body) }))
		}
		if e.Else != nil {
			b.WriteString(p.pad() + "else\n")
			b.WriteString(p.sub(func() { p.stmts(e.Else) }))
		}
		b.WriteString(p.pad() + "end if")
		return b.String()
	case *syntax.MatchExpr:
		var b strings.Builder
		if e.Values != nil {
			vals := make([]string, len(e.Values))
			for i, v := range e.Values {
				vals[i] = p.expr(v)
			}
			b.WriteString("match " + strings.Join(vals, ", ") + "\n")
		} else {
			b.WriteString("match " + p.expr(e.X) + "\n")
		}
		b.WriteString(p.sub(func() {
			for _, arm := range e.Arms {
				p.comments(arm.Leading)
				head := "case " + Pattern(arm.Pattern)
				if arm.Guard != nil {
					head += " if " + p.expr(arm.Guard)
				}
				if x := simpleBody(arm.Body); x != nil {
					p.line(head + " => " + p.expr(x) + trail(arm.Trailing))
					continue
				}
				if st := simpleStmt(arm.Body); st != nil {
					p.line(head + " => " + p.stmt(st) + trail(arm.Trailing))
					continue
				}
				p.line(head + trail(arm.Trailing))
				p.indent++
				p.stmts(arm.Body)
				p.indent--
			}
			p.comments(e.EndComments)
		}))
		b.WriteString(p.pad() + "end match")
		return b.String()
	case *syntax.ForExpr:
		return "for " + e.Var + " in " + p.expr(e.Iter) + "\n" + p.sub(func() { p.stmts(e.Body) }) + p.pad() + "end for"
	case *syntax.WhileExpr:
		return "while " + p.expr(e.Cond) + "\n" + p.sub(func() { p.stmts(e.Body) }) + p.pad() + "end while"
	case *syntax.ReturnExpr:
		if e.X == nil {
			return "return"
		}
		return "return " + p.expr(e.X)
	case *syntax.BreakExpr:
		return "break"
	case *syntax.ContinueExpr:
		return "continue"
	case *syntax.TodoExpr:
		return "???"
	}
	return "?"
}

// simpleBody returns the single expression of a block that can be written
// in the short `=> expr` form, or nil.
func simpleBody(b *syntax.Block) syntax.Expr {
	if len(b.Stmts) != 1 || len(b.EndComments) > 0 {
		return nil
	}
	es, ok := b.Stmts[0].(*syntax.ExprStmt)
	if !ok || len(es.Leading) > 0 || es.Trailing != "" {
		return nil
	}
	if multiline(es.X) {
		return nil
	}
	return es.X
}

// simpleStmt returns the single set/expect statement of an arm body that
// can be written in the short form, or nil.
func simpleStmt(b *syntax.Block) syntax.Stmt {
	if len(b.Stmts) != 1 || len(b.EndComments) > 0 {
		return nil
	}
	st := b.Stmts[0]
	m := st.M()
	if len(m.Leading) > 0 || m.Trailing != "" {
		return nil
	}
	switch x := st.(type) {
	case *syntax.SetStmt:
		if !multiline(x.Value) {
			return st
		}
	case *syntax.ExpectStmt:
		if !multiline(x.X) {
			return st
		}
	}
	return nil
}

// multiline reports whether an expression contains a block construct.
func multiline(e syntax.Expr) bool {
	switch e := e.(type) {
	case *syntax.IfExpr, *syntax.MatchExpr, *syntax.ForExpr, *syntax.WhileExpr:
		return true
	case *syntax.LambdaExpr:
		return simpleBody(e.Body) == nil
	case *syntax.CallExpr:
		if multiline(e.Fn) {
			return true
		}
		for _, a := range e.Args {
			if multiline(a.Value) {
				return true
			}
		}
	case *syntax.PipeExpr:
		return multiline(e.L) || multiline(e.R)
	case *syntax.BinaryExpr:
		return multiline(e.L) || multiline(e.R)
	case *syntax.ParenExpr:
		return multiline(e.X)
	case *syntax.UnaryExpr:
		return multiline(e.X)
	case *syntax.TryExpr:
		return multiline(e.X)
	case *syntax.FieldExpr:
		return multiline(e.X)
	case *syntax.ReturnExpr:
		return e.X != nil && multiline(e.X)
	case *syntax.ListLit:
		for _, x := range e.Elems {
			if multiline(x) {
				return true
			}
		}
	case *syntax.MapLit:
		for _, en := range e.Entries {
			if multiline(en.Key) || multiline(en.Value) {
				return true
			}
		}
	case *syntax.RecordLit:
		if e.Base != nil && multiline(e.Base) {
			return true
		}
		for _, f := range e.Fields {
			if multiline(f.Value) {
				return true
			}
		}
	}
	return false
}

// list renders a bracketed, comma-separated list, one item per line when
// it would be too long or an item spans lines.
func (p *printer) list(open, close string, items []string) string {
	one := open + strings.Join(items, ", ") + close
	if len(p.pad())+len(one) <= maxWidth && !strings.Contains(one, "\n") {
		return one
	}
	var b strings.Builder
	b.WriteString(open + "\n")
	inner := p.pad() + "  "
	for _, it := range items {
		// Re-indent continuation lines of nested multi-line items.
		it = strings.ReplaceAll(it, "\n", "\n  ")
		b.WriteString(inner + it + ",\n")
	}
	b.WriteString(p.pad() + close)
	return b.String()
}

func (p *printer) str(e *syntax.StrLit) string {
	if e.Raw {
		content := e.Segs[0].Lit
		if !strings.Contains(content, "\n") && !strings.Contains(content, `"""`) && !strings.HasPrefix(content, `"`) && !strings.HasSuffix(content, `"`) {
			return `"""` + content + `"""`
		}
		var b strings.Builder
		b.WriteString(`"""` + "\n")
		ind := p.pad() + "  "
		for _, l := range strings.Split(content, "\n") {
			if l == "" {
				b.WriteString("\n")
			} else {
				b.WriteString(ind + l + "\n")
			}
		}
		b.WriteString(ind + `"""`)
		return b.String()
	}
	var b strings.Builder
	b.WriteByte('"')
	for i, s := range e.Segs {
		if s.Expr != nil {
			b.WriteString("${" + p.expr(s.Expr) + "}")
			continue
		}
		nextInterp := i+1 < len(e.Segs) && e.Segs[i+1].Expr != nil
		b.WriteString(escape(s.Lit, nextInterp))
	}
	b.WriteByte('"')
	return b.String()
}

func escape(s string, nextInterp bool) string {
	var b strings.Builder
	rs := []rune(s)
	for i, r := range rs {
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
		case 0:
			b.WriteString(`\0`)
		case '$':
			if (i+1 < len(rs) && rs[i+1] == '{') || (i+1 == len(rs) && nextInterp) {
				b.WriteString(`\$`)
			} else {
				b.WriteByte('$')
			}
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func quoteStr(s string) string { return `"` + escape(s, false) + `"` }

// Pattern renders a pattern.
func Pattern(pt syntax.Pattern) string {
	switch pt := pt.(type) {
	case *syntax.WildPat:
		return "_"
	case *syntax.BindPat:
		return pt.Name
	case *syntax.LitPat:
		p := &printer{}
		return p.expr(pt.Value)
	case *syntax.CtorPat:
		if pt.Tuple {
			as := make([]string, len(pt.Args))
			allWild := true
			for i, a := range pt.Args {
				as[i] = Pattern(a)
				if _, ok := a.(*syntax.WildPat); !ok {
					allWild = false
				}
			}
			if allWild {
				return "_"
			}
			return strings.Join(as, ", ")
		}
		s := pt.Name
		if pt.Module != "" {
			s = pt.Module + "." + s
		}
		if len(pt.Args) > 0 {
			var as []string
			for _, a := range pt.Args {
				as = append(as, Pattern(a))
			}
			s += "(" + strings.Join(as, ", ") + ")"
		}
		return s
	case *syntax.OrPat:
		var as []string
		for _, a := range pt.Alts {
			as = append(as, Pattern(a))
		}
		return strings.Join(as, " | ")
	case *syntax.ListPat:
		var as []string
		for _, a := range pt.Elems {
			as = append(as, Pattern(a))
		}
		if pt.HasRest {
			r := pt.Rest
			if r == "" {
				r = "_"
			}
			as = append(as, ".."+r)
			for _, a := range pt.Suffix {
				as = append(as, Pattern(a))
			}
		}
		return "[" + strings.Join(as, ", ") + "]"
	}
	return "_"
}
