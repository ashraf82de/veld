package syntax

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ashraf82de/veld/internal/diag"
)

// Effects is the fixed set of effect names a function may declare.
var Effects = []string{"io", "fs", "net", "env", "time", "rand"}

func IsEffect(s string) bool {
	for _, e := range Effects {
		if e == s {
			return true
		}
	}
	return false
}

type parser struct {
	file     string
	toks     []Token
	pos      int
	comments []Comment
	ci       int
	diags    *diag.List
	errLine  int // last line an error was reported on (one error per line)
}

type bailout struct{}

// Parse parses a whole source file.
func Parse(file, src string, diags *diag.List) *File {
	toks, comments := Lex(file, src, diags)
	p := &parser{file: file, toks: toks, comments: comments, diags: diags}
	f := p.parseFile()
	f.Source = src
	return f
}

// ParseExpr parses a standalone expression (used for string interpolation).
func parseExprAt(file, src string, start diag.Pos, diags *diag.List) Expr {
	toks, _ := lexAt(file, src, start, diags)
	p := &parser{file: file, toks: toks, diags: diags}
	var e Expr
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(bailout); !ok {
					panic(r)
				}
			}
		}()
		p.skipNewlines()
		e = p.parseExpr()
		p.skipNewlines()
		if p.peek().Kind != EOF {
			p.errAt(p.peek().Span, "E104", "unexpected %s in interpolation", p.describe(p.peek()))
		}
	}()
	if e == nil {
		e = &TodoExpr{Span: diag.Span{File: file, Start: start, End: start}}
	}
	return e
}

// ---- token helpers ----

func (p *parser) peek() Token { return p.toks[p.pos] }
func (p *parser) peekN(n int) Token {
	if p.pos+n < len(p.toks) {
		return p.toks[p.pos+n]
	}
	return p.toks[len(p.toks)-1]
}
func (p *parser) at(k Kind) bool { return p.toks[p.pos].Kind == k }
func (p *parser) next() Token {
	t := p.toks[p.pos]
	if t.Kind != EOF {
		p.pos++
	}
	return t
}
func (p *parser) prevEnd() diag.Pos {
	if p.pos == 0 {
		return p.toks[0].Span.Start
	}
	return p.toks[p.pos-1].Span.End
}

func (p *parser) span(start diag.Pos) diag.Span {
	return diag.Span{File: p.file, Start: start, End: p.prevEnd()}
}

func (p *parser) describe(t Token) string {
	switch t.Kind {
	case IDENT:
		return fmt.Sprintf("identifier `%s`", t.Text)
	case INT, FLOAT:
		return fmt.Sprintf("number `%s`", t.Text)
	case STRING:
		return "string literal"
	case NEWLINE:
		return "end of line"
	case EOF:
		return "end of file"
	}
	return "`" + t.Kind.String() + "`"
}

func (p *parser) errAt(sp diag.Span, code, format string, args ...any) *diag.Diagnostic {
	if sp.Start.Line == p.errLine {
		return &diag.Diagnostic{} // suppress cascades on the same line
	}
	p.errLine = sp.Start.Line
	return p.diags.Errorf(code, sp, format, args...)
}

func (p *parser) fail(sp diag.Span, code, format string, args ...any) *diag.Diagnostic {
	return p.errAt(sp, code, format, args...)
}

func (p *parser) expect(k Kind, context string) Token {
	if k == RPAREN || k == RBRACK || k == RBRACE {
		p.skipNewlines()
	}
	if p.at(k) {
		return p.next()
	}
	t := p.peek()
	d := p.errAt(t.Span, "E104", "expected `%s` %s, found %s", k.String(), context, p.describe(t))
	if k == RPAREN || k == RBRACK || k == RBRACE {
		d.WithFix("insert `"+k.String()+"`", diag.Span{File: p.file, Start: p.prevEnd(), End: p.prevEnd()}, k.String())
	}
	panic(bailout{})
}

func (p *parser) expectIdent(context string) Token {
	if p.at(IDENT) {
		return p.next()
	}
	t := p.peek()
	d := p.errAt(t.Span, "E104", "expected a name %s, found %s", context, p.describe(t))
	if t.Kind.IsKeyword() {
		d.Note("`%s` is a reserved keyword and cannot be used as a name", t.Text)
	}
	panic(bailout{})
}

func (p *parser) skipNewlines() {
	for p.at(NEWLINE) {
		p.next()
	}
}

// endLine requires the end of a statement: a newline, or a following block
// terminator (which is left in place).
func (p *parser) endLine(what string) {
	switch p.peek().Kind {
	case NEWLINE:
		p.skipNewlines()
	case EOF:
	default:
		t := p.peek()
		p.errAt(t.Span, "E105", "expected end of line after %s, found %s", what, p.describe(t)).
			Note("each statement goes on its own line")
		panic(bailout{})
	}
}

// syncLine skips to the start of the next line.
func (p *parser) syncLine() {
	for !p.at(NEWLINE) && !p.at(EOF) {
		p.next()
	}
	p.skipNewlines()
}

// lastLine is the line of the last real token before the current one.
func (p *parser) lastLine() int {
	for i := p.pos - 1; i >= 0; i-- {
		if p.toks[i].Kind != NEWLINE {
			return p.toks[i].Span.End.Line
		}
	}
	return 0
}

// ---- comments ----

// leading takes the comments before line. An empty string marks a blank
// line between comments, or between the last comment and the item.
func (p *parser) leading(line int) []string {
	var out []string
	for p.ci < len(p.comments) && p.comments[p.ci].Line < line {
		c := p.comments[p.ci]
		out = append(out, c.Text)
		p.ci++
		next := line
		if p.ci < len(p.comments) && p.comments[p.ci].Line < line {
			next = p.comments[p.ci].Line
		}
		if next > c.Line+1 {
			out = append(out, "")
		}
	}
	return out
}

func (p *parser) trailing(line int) string {
	if p.ci < len(p.comments) && p.comments[p.ci].Line == line && !p.comments[p.ci].Own {
		c := p.comments[p.ci].Text
		p.ci++
		return c
	}
	return ""
}

func splitDoc(lead []string) (comments, doc []string) {
	// Doc comments are the trailing run of "##" comments right before the decl.
	i := len(lead)
	for i > 0 && strings.HasPrefix(lead[i-1], "##") {
		i--
	}
	for _, d := range lead[i:] {
		doc = append(doc, strings.TrimSpace(strings.TrimPrefix(d, "##")))
	}
	return lead[:i], doc
}

// ---- file level ----

func (p *parser) parseFile() *File {
	f := &File{Path: p.file}
	p.skipNewlines()
	for !p.at(EOF) {
		start := p.pos
		func() {
			defer func() {
				if r := recover(); r != nil {
					if _, ok := r.(bailout); !ok {
						panic(r)
					}
					p.syncDecl()
				}
			}()
			line := p.peek().Span.Start.Line
			lead := p.leading(line)
			if p.at(KW_USE) {
				u := p.parseUse()
				u.Leading = lead
				if len(f.Decls) > 0 {
					p.diags.Errorf("E106", u.Span, "`use` must appear before all declarations").
						Note("move this line to the top of the file")
				}
				f.Uses = append(f.Uses, u)
				return
			}
			d := p.parseDecl(lead)
			if d != nil {
				f.Decls = append(f.Decls, d)
			}
		}()
		if p.pos == start { // no progress; avoid infinite loop
			p.next()
		}
		p.skipNewlines()
	}
	f.EndComments = p.leading(1 << 30)
	return f
}

// syncDecl skips to the next line that starts a top-level declaration.
func (p *parser) syncDecl() {
	for !p.at(EOF) {
		if p.at(NEWLINE) {
			p.next()
			switch p.peek().Kind {
			case KW_FN, KW_TYPE, KW_RECORD, KW_TEST, KW_PUB, KW_USE, KW_EXTERN:
				if p.peek().Span.Start.Col == 1 {
					return
				}
			}
			continue
		}
		p.next()
	}
}

func (p *parser) parseUse() *Use {
	start := p.next().Span.Start
	var path []string
	path = append(path, p.expectIdent("after `use`").Text)
	for p.at(DOT) {
		p.next()
		path = append(path, p.expectIdent("in module path").Text)
	}
	u := &Use{Path: path, Span: p.span(start)}
	u.Trailing = p.trailing(u.Span.End.Line)
	p.endLine("use")
	return u
}

func (p *parser) parseDecl(lead []string) Decl {
	lead, doc := splitDoc(lead)
	start := p.peek().Span.Start
	pub, extern := false, false
	if p.at(KW_PUB) {
		p.next()
		pub = true
	}
	if p.at(KW_EXTERN) {
		p.next()
		extern = true
	}
	switch p.peek().Kind {
	case KW_FN:
		d := p.parseFnDecl(start, pub, extern)
		d.Leading, d.Doc = lead, doc
		return d
	case KW_TYPE:
		d := p.parseTypeDecl(start, pub)
		d.Leading, d.Doc = lead, doc
		return d
	case KW_RECORD:
		d := p.parseRecordDecl(start, pub)
		d.Leading, d.Doc = lead, doc
		return d
	case KW_TEST:
		if pub {
			p.errAt(p.peek().Span, "E107", "tests cannot be `pub`")
		}
		d := p.parseTest(start)
		d.Leading = lead
		return d
	}
	t := p.peek()
	d := p.errAt(t.Span, "E107", "expected a declaration (`fn`, `type`, `record`, `test` or `use`), found %s", p.describe(t))
	if t.Kind == KW_LET || t.Kind == KW_VAR {
		d.Note("Veld has no global variables; define a zero-argument `fn` that returns the value instead")
	}
	if t.Kind == IDENT && (t.Text == "def" || t.Text == "func" || t.Text == "function") {
		d.WithFix("use `fn`", t.Span, "fn")
	}
	if t.Kind == IDENT && (t.Text == "struct" || t.Text == "class") {
		d.WithFix("use `record`", t.Span, "record")
	}
	if t.Kind == IDENT && t.Text == "enum" {
		d.WithFix("use `type`", t.Span, "type")
	}
	panic(bailout{})
}

func (p *parser) parseTParams() []string {
	var out []string
	if !p.at(LBRACK) {
		return nil
	}
	p.next()
	for p.skipNewlines(); !p.at(RBRACK); p.skipNewlines() {
		out = append(out, p.expectIdent("as type parameter").Text)
		if p.skipNewlines(); !p.at(COMMA) {
			break
		}
		p.next()
	}
	p.expect(RBRACK, "to close type parameters")
	return out
}

func (p *parser) parseParams(requireTypes bool) []*Param {
	p.expect(LPAREN, "to start parameters")
	var out []*Param
	for p.skipNewlines(); !p.at(RPAREN); p.skipNewlines() {
		t := p.expectIdent("as parameter")
		prm := &Param{Name: t.Text, Span: t.Span}
		if p.at(COLON) {
			p.next()
			prm.Type = p.parseType()
			prm.Span = diag.Join(t.Span, prm.Type.Sp())
		} else if requireTypes {
			p.errAt(p.peek().Span, "E108", "parameter `%s` needs a type annotation", t.Text).
				WithFix("annotate the parameter", diag.Span{File: p.file, Start: t.Span.End, End: t.Span.End}, ": T").
				Note("function signatures are always fully typed in Veld")
		}
		out = append(out, prm)
		if p.skipNewlines(); !p.at(COMMA) {
			break
		}
		p.next()
	}
	p.expect(RPAREN, "to close parameters")
	return out
}

func (p *parser) parseEffects() []string {
	var out []string
	for {
		t := p.expectIdent("as effect name")
		if !IsEffect(t.Text) {
			d := p.errAt(t.Span, "E109", "unknown effect `%s`", t.Text).Note("effects are: %s", strings.Join(Effects, ", "))
			if s := diag.Suggest(t.Text, Effects); s != "" {
				d.WithFix("use `"+s+"`", t.Span, s)
			}
		}
		out = append(out, t.Text)
		// A comma continues the list only if followed by another effect name
		// (inside a parameter list the comma may separate parameters).
		if p.at(COMMA) && p.peekN(1).Kind == IDENT && IsEffect(p.peekN(1).Text) && p.peekN(2).Kind != COLON {
			p.next()
			continue
		}
		return out
	}
}

func (p *parser) parseFnDecl(start diag.Pos, pub, extern bool) *FnDecl {
	p.next() // fn
	name := p.expectIdent("after `fn`")
	d := &FnDecl{Pub: pub, Extern: extern, Name: name.Text, NameSpan: name.Span}
	d.TParams = p.parseTParams()
	d.Params = p.parseParams(true)
	if p.at(ARROW) {
		p.next()
		d.Ret = p.parseType()
	} else if p.at(COLON) {
		p.errAt(p.peek().Span, "E108", "return types are written with `->`").
			WithFix("use `->`", p.peek().Span, " ->")
		p.next()
		d.Ret = p.parseType()
	}
	// A missing return type is reported by the checker, which can infer it.
	if p.at(KW_USES) {
		p.next()
		d.Effects = p.parseEffects()
	}
	d.HeadEnd = p.prevEnd()
	headLine := d.HeadEnd.Line
	if extern {
		d.Span = p.span(start)
		d.Trailing = p.trailing(headLine)
		p.endLine("extern fn signature")
		return d
	}
	if !p.at(NEWLINE) {
		t := p.peek()
		dd := p.errAt(t.Span, "E105", "expected a new line after the signature of `%s`, found %s", d.Name, p.describe(t))
		if t.Kind == FATARROW || t.Kind == ASSIGN || t.Kind == LBRACE || t.Kind == COLON {
			dd.Note("function bodies start on the next line and end with `end fn`")
		}
		panic(bailout{})
	}
	d.Trailing = p.trailing(headLine)
	p.skipNewlines()
	for p.at(KW_REQUIRES) || p.at(KW_ENSURES) {
		isReq := p.at(KW_REQUIRES)
		p.next()
		e := p.parseExpr()
		if isReq {
			d.Requires = append(d.Requires, e)
		} else {
			d.Ensures = append(d.Ensures, e)
		}
		p.endLine("contract clause")
	}
	d.Body = p.parseBlock(KW_END)
	p.parseEnd(KW_FN, "fn", start)
	d.Span = p.span(start)
	return d
}

// parseEnd consumes `end <kw>`.
func (p *parser) parseEnd(kw Kind, what string, open diag.Pos) {
	if !p.at(KW_END) {
		t := p.peek()
		p.errAt(t.Span, "E110", "expected `end %s` to close the `%s` opened on line %d, found %s", what, what, open.Line, p.describe(t)).
			WithFix("insert `end "+what+"`", diag.Span{File: p.file, Start: t.Span.Start, End: t.Span.Start}, "end "+what+"\n")
		panic(bailout{})
	}
	endTok := p.next()
	if !p.at(kw) {
		t := p.peek()
		d := p.errAt(t.Span, "E110", "expected `end %s` to close the `%s` opened on line %d", what, what, open.Line)
		if t.Kind.IsKeyword() && t.Kind != KW_END {
			d.Message = fmt.Sprintf("mismatched block closer: found `end %s` but the innermost open block is the `%s` on line %d", t.Text, what, open.Line)
			d.WithFix("change to `end "+what+"`", t.Span, what)
		} else {
			d.WithFix("write `end "+what+"`", diag.Span{File: p.file, Start: endTok.Span.End, End: endTok.Span.End}, " "+what)
		}
		if t.Kind.IsKeyword() {
			p.next()
		}
		return
	}
	p.next()
}

func (p *parser) parseTypeDecl(start diag.Pos, pub bool) *TypeDecl {
	p.next()
	name := p.expectIdent("after `type`")
	d := &TypeDecl{Pub: pub, Name: name.Text}
	d.TParams = p.parseTParams()
	if p.at(ASSIGN) {
		p.errAt(p.peek().Span, "E111", "type declarations list their variants on the following lines").
			Note("write:\ntype Shape\n  case Circle(radius: Float)\n  case Square(side: Float)\nend type")
		panic(bailout{})
	}
	d.Trailing = p.trailing(p.prevEnd().Line)
	p.endLine("type header")
	for p.at(KW_CASE) {
		vs := p.peek().Span.Start
		lead := p.leading(vs.Line)
		p.next()
		vn := p.expectIdent("as variant name")
		v := &Variant{Name: vn.Text}
		v.Leading = lead
		if p.at(LPAREN) {
			p.next()
			for p.skipNewlines(); !p.at(RPAREN); p.skipNewlines() {
				fs := p.peek().Span.Start
				fname := p.expectIdent("as variant field name")
				if !p.at(COLON) {
					p.errAt(p.peek().Span, "E111", "variant fields are named: `%s: Type`", fname.Text).
						Note("write e.g. `case Circle(radius: Float)`")
					panic(bailout{})
				}
				p.next()
				ft := p.parseType()
				v.Fields = append(v.Fields, &Field{Name: fname.Text, Type: ft, Span: p.span(fs)})
				if p.skipNewlines(); !p.at(COMMA) {
					break
				}
				p.next()
			}
			p.expect(RPAREN, "to close variant fields")
		}
		v.Span = p.span(vs)
		v.Trailing = p.trailing(v.Span.End.Line)
		p.endLine("variant")
		d.Variants = append(d.Variants, v)
	}
	if p.at(IDENT) && IsUpper(p.peek().Text) {
		t := p.peek()
		p.errAt(t.Span, "E111", "each variant starts with `case`").
			WithFix("insert `case`", diag.Span{File: p.file, Start: t.Span.Start, End: t.Span.Start}, "case ")
		panic(bailout{})
	}
	d.EndComments = p.leading(p.peek().Span.Start.Line)
	if len(d.Variants) == 0 {
		p.errAt(name.Span, "E111", "type `%s` has no variants", d.Name).Note("use `record` for a product type")
	}
	p.parseEnd(KW_TYPE, "type", start)
	d.Span = p.span(start)
	return d
}

func (p *parser) parseRecordDecl(start diag.Pos, pub bool) *RecordDecl {
	p.next()
	name := p.expectIdent("after `record`")
	d := &RecordDecl{Pub: pub, Name: name.Text}
	d.TParams = p.parseTParams()
	d.Trailing = p.trailing(p.prevEnd().Line)
	p.endLine("record header")
	for p.at(IDENT) {
		fs := p.peek().Span.Start
		lead := p.leading(fs.Line)
		fname := p.next()
		p.expect(COLON, "after field name")
		ft := p.parseType()
		f := &Field{Name: fname.Text, Type: ft, Span: p.span(fs)}
		f.Leading = lead
		f.Trailing = p.trailing(f.Span.End.Line)
		if p.at(COMMA) {
			p.errAt(p.peek().Span, "E105", "record fields go one per line, without commas").
				WithFix("remove the comma", p.peek().Span, "")
			p.next()
		}
		p.endLine("record field")
		d.Fields = append(d.Fields, f)
	}
	d.EndComments = p.leading(p.peek().Span.Start.Line)
	p.parseEnd(KW_RECORD, "record", start)
	d.Span = p.span(start)
	return d
}

func (p *parser) parseTest(start diag.Pos) *TestDecl {
	p.next()
	if !p.at(STRING) {
		p.errAt(p.peek().Span, "E104", "expected a test name string after `test`").
			Note(`write: test "adds two numbers"`)
		panic(bailout{})
	}
	nameTok := p.next()
	name := ""
	for _, part := range nameTok.Parts {
		name += part.Lit
	}
	d := &TestDecl{Name: name}
	d.Trailing = p.trailing(nameTok.Span.End.Line)
	p.endLine("test name")
	d.Body = p.parseBlock(KW_END)
	p.parseEnd(KW_TEST, "test", start)
	d.Span = p.span(start)
	return d
}

// ---- types ----

func (p *parser) parseType() TypeExpr {
	start := p.peek().Span.Start
	t := p.expectIdent("as type")
	if t.Text == "Fn" && p.at(LPAREN) {
		p.next()
		ft := &FnType{}
		for p.skipNewlines(); !p.at(RPAREN); p.skipNewlines() {
			ft.Params = append(ft.Params, p.parseType())
			if p.skipNewlines(); !p.at(COMMA) {
				break
			}
			p.next()
		}
		p.expect(RPAREN, "to close function type parameters")
		p.expect(ARROW, "in function type (write `Fn(A) -> B`)")
		ft.Ret = p.parseType()
		if p.at(KW_USES) {
			p.next()
			ft.Effects = p.parseEffects()
		}
		ft.Span = p.span(start)
		return ft
	}
	nt := &NamedType{Name: t.Text}
	if p.at(DOT) && !IsUpper(t.Text) {
		p.next()
		nt.Module = t.Text
		nt.Name = p.expectIdent("as type name").Text
	}
	if p.at(LBRACK) {
		p.next()
		for p.skipNewlines(); !p.at(RBRACK); p.skipNewlines() {
			nt.Args = append(nt.Args, p.parseType())
			if p.skipNewlines(); !p.at(COMMA) {
				break
			}
			p.next()
		}
		p.expect(RBRACK, "to close type arguments")
	} else if p.at(LT) {
		p.errAt(p.peek().Span, "E112", "type arguments use square brackets: `%s[T]`", nt.Name)
		panic(bailout{})
	}
	nt.Span = p.span(start)
	return nt
}

// ---- blocks and statements ----

// parseBlock parses statements until one of the terminator keywords.
func (p *parser) parseBlock(terms ...Kind) *Block {
	b := &Block{}
	start := p.peek().Span.Start
	isTerm := func() bool {
		if p.at(EOF) {
			return true
		}
		for _, k := range terms {
			if p.at(k) {
				return true
			}
		}
		return false
	}
	p.skipNewlines()
	for !isTerm() {
		before := p.pos
		func() {
			defer func() {
				if r := recover(); r != nil {
					if _, ok := r.(bailout); !ok {
						panic(r)
					}
					p.syncLine()
				}
			}()
			s := p.parseStmt()
			if s != nil {
				b.Stmts = append(b.Stmts, s)
			}
		}()
		if p.pos == before {
			p.next()
		}
		p.skipNewlines()
	}
	b.EndComments = p.leading(p.peek().Span.Start.Line)
	b.Span = diag.Span{File: p.file, Start: start, End: p.prevEnd()}
	return b
}

// parseSimpleStmt parses a `set` or `expect` statement without its line end.
func (p *parser) parseSimpleStmt() Stmt {
	t := p.peek()
	start := t.Span.Start
	if t.Kind == KW_EXPECT {
		p.next()
		es := &ExpectStmt{X: p.parseExpr()}
		es.Span = p.span(start)
		return es
	}
	p.next()
	name := p.expectIdent("after `set`")
	if p.at(DOT) {
		p.errAt(p.peek().Span, "E113", "records are immutable; `set` only reassigns a `var`").
			Note("build an updated copy instead: set x = Point{..x, y: 2}")
		panic(bailout{})
	}
	ss := &SetStmt{Name: name.Text, NameSpan: name.Span}
	p.expect(ASSIGN, "after `set "+name.Text+"`")
	ss.Value = p.parseExpr()
	ss.Span = p.span(start)
	return ss
}

func (p *parser) parseStmt() Stmt {
	t := p.peek()
	first := t.Span.Start.Line
	if p.ci < len(p.comments) && p.comments[p.ci].Line < first {
		first = p.comments[p.ci].Line
	}
	blank := p.pos > 0 && first-p.lastLine() > 1
	lead := p.leading(t.Span.Start.Line)
	start := t.Span.Start
	var s Stmt
	switch t.Kind {
	case KW_LET, KW_VAR:
		p.next()
		name := p.expectIdent("after `" + t.Text + "`")
		ls := &LetStmt{Mutable: t.Kind == KW_VAR, Name: name.Text, NameSpan: name.Span}
		if p.at(COLON) {
			p.next()
			ls.Type = p.parseType()
		}
		if !p.at(ASSIGN) {
			p.errAt(p.peek().Span, "E104", "expected `=` in `%s %s`, found %s", t.Text, name.Text, p.describe(p.peek())).
				Note("every binding must be initialized")
			panic(bailout{})
		}
		p.next()
		ls.Value = p.parseExpr()
		ls.Span = p.span(start)
		s = ls
	case KW_SET, KW_EXPECT:
		s = p.parseSimpleStmt()
	default:
		x := p.parseExpr()
		if p.at(ASSIGN) {
			d := p.errAt(p.peek().Span, "E113", "assignment needs `set` (for an existing `var`) or `let`/`var` (for a new binding)")
			if id, ok := x.(*Ident); ok {
				d.WithFix("reassign with `set`", diag.Span{File: p.file, Start: id.Span.Start, End: id.Span.Start}, "set ")
			}
			panic(bailout{})
		}
		if p.at(PLUS) || p.at(MINUS) {
			if n := p.peekN(1); n.Kind == ASSIGN {
				p.errAt(p.peek().Span, "E113", "compound assignment is not supported").
					Note("write `set x = x + 1`")
				panic(bailout{})
			}
		}
		s = &ExprStmt{X: x, Span: p.span(start)}
	}
	m := s.M()
	m.Leading = lead
	m.Blank = blank
	m.Trailing = p.trailing(p.prevEnd().Line)
	switch p.peek().Kind {
	case NEWLINE:
		p.skipNewlines()
	case EOF, KW_END, KW_ELSE, KW_CASE:
	default:
		nt := p.peek()
		d := p.errAt(nt.Span, "E105", "expected end of line, found %s", p.describe(nt))
		if nt.Kind == LBRACK {
			d.Message = "Veld has no `[]` indexing"
			d.Note("use `list.get(xs, i)` which returns Option[T], or `map.get(m, k)`")
		} else if nt.Kind == LBRACE {
			d.Note("blocks are not delimited by braces; start the block on a new line and close it with `end`")
		}
		panic(bailout{})
	}
	return s
}

// ---- expressions ----

// Precedence, loosest first: and/or, not, comparisons, |>, + -, * / %,
// unary -, postfix (call, field, ?).
func (p *parser) parseExpr() Expr { return p.parseLogic() }

func (p *parser) parsePipe() Expr {
	l := p.parseAdd()
	for p.at(PIPE) {
		p.next()
		r := p.parseAdd()
		// `x |> f()?` means `(x |> f())?`.
		if tr, ok := r.(*TryExpr); ok {
			if call, ok := tr.X.(*CallExpr); ok {
				l = &TryExpr{X: &PipeExpr{L: l, R: call, Span: diag.Join(l.Sp(), call.Span)}, Span: diag.Join(l.Sp(), tr.Span)}
				continue
			}
		}
		call, ok := r.(*CallExpr)
		if !ok {
			p.errAt(r.Sp(), "E114", "the right side of `|>` must be a call like `f(x)`; the piped value becomes its first argument")
			panic(bailout{})
		}
		l = &PipeExpr{L: l, R: call, Span: diag.Join(l.Sp(), call.Span)}
	}
	return l
}

func (p *parser) parseLogic() Expr {
	l := p.parseNot()
	var op Kind
	for p.at(KW_AND) || p.at(KW_OR) {
		t := p.next()
		if op != 0 && op != t.Kind {
			p.errAt(t.Span, "E115", "mixing `and` and `or` requires parentheses").
				Note("write `(a and b) or c` or `a and (b or c)`")
		}
		op = t.Kind
		r := p.parseNot()
		l = &BinaryExpr{Op: t.Kind, L: l, R: r, OpSpan: t.Span, Span: diag.Join(l.Sp(), r.Sp())}
	}
	return l
}

func (p *parser) parseNot() Expr {
	if p.at(KW_NOT) {
		t := p.next()
		x := p.parseNot()
		return &UnaryExpr{Op: KW_NOT, X: x, Span: diag.Join(t.Span, x.Sp())}
	}
	return p.parseCmp()
}

func isCmp(k Kind) bool {
	switch k {
	case EQ, NE, LT, LE, GT, GE:
		return true
	}
	return false
}

func (p *parser) parseCmp() Expr {
	l := p.parsePipe()
	if isCmp(p.peek().Kind) {
		t := p.next()
		r := p.parsePipe()
		l = &BinaryExpr{Op: t.Kind, L: l, R: r, OpSpan: t.Span, Span: diag.Join(l.Sp(), r.Sp())}
		if isCmp(p.peek().Kind) {
			p.errAt(p.peek().Span, "E115", "comparisons cannot be chained").
				Note("write `a < b and b < c`")
		}
	}
	return l
}

func (p *parser) parseAdd() Expr {
	l := p.parseMul()
	for p.at(PLUS) || p.at(MINUS) {
		t := p.next()
		r := p.parseMul()
		l = &BinaryExpr{Op: t.Kind, L: l, R: r, OpSpan: t.Span, Span: diag.Join(l.Sp(), r.Sp())}
	}
	return l
}

func (p *parser) parseMul() Expr {
	l := p.parseUnary()
	for p.at(STAR) || p.at(SLASH) || p.at(PERCENT) {
		t := p.next()
		r := p.parseUnary()
		l = &BinaryExpr{Op: t.Kind, L: l, R: r, OpSpan: t.Span, Span: diag.Join(l.Sp(), r.Sp())}
	}
	return l
}

func (p *parser) parseUnary() Expr {
	if p.at(MINUS) {
		t := p.next()
		x := p.parseUnary()
		// Fold negative literals so they print and match canonically.
		switch lit := x.(type) {
		case *IntLit:
			if lit.Text != "" && lit.Text[0] != '-' {
				return &IntLit{Value: -lit.Value, Text: "-" + lit.Text, Span: diag.Join(t.Span, lit.Span)}
			}
		case *FloatLit:
			if lit.Text != "" && lit.Text[0] != '-' {
				return &FloatLit{Value: -lit.Value, Text: "-" + lit.Text, Span: diag.Join(t.Span, lit.Span)}
			}
		}
		return &UnaryExpr{Op: MINUS, X: x, Span: diag.Join(t.Span, x.Sp())}
	}
	return p.parsePostfix()
}

func (p *parser) parsePostfix() Expr {
	x := p.parsePrimary()
	for {
		switch p.peek().Kind {
		case LPAREN:
			x = p.parseCall(x)
		case DOT:
			p.next()
			nt := p.peek()
			if nt.Kind != IDENT && !nt.Kind.IsKeyword() {
				p.errAt(nt.Span, "E104", "expected a field or function name after `.`, found %s", p.describe(nt))
				panic(bailout{})
			}
			p.next()
			// qualified record literal: mod.Rec{...}
			if id, ok := x.(*Ident); ok && !IsUpper(id.Name) && IsUpper(nt.Text) && p.at(LBRACE) {
				x = p.parseRecordLit(id.Name, nt.Text, id.Span.Start)
				continue
			}
			x = &FieldExpr{X: x, Name: nt.Text, NameSpan: nt.Span, Span: diag.Join(x.Sp(), nt.Span)}
		case QUESTION:
			t := p.next()
			x = &TryExpr{X: x, Span: diag.Join(x.Sp(), t.Span)}
		default:
			return x
		}
	}
}

func (p *parser) parseCall(fn Expr) *CallExpr {
	lp := p.next()
	c := &CallExpr{Fn: fn, LParen: lp.Span.Start}
	for p.skipNewlines(); !p.at(RPAREN); p.skipNewlines() {
		a := &Arg{}
		if p.at(IDENT) && p.peekN(1).Kind == COLON {
			nt := p.next()
			p.next()
			a.Name, a.NameSpan = nt.Text, nt.Span
		} else if p.at(IDENT) && p.peekN(1).Kind == ASSIGN {
			nt := p.peek()
			p.errAt(p.peekN(1).Span, "E116", "named arguments use `:`").
				WithFix("write `"+nt.Text+": value`", p.peekN(1).Span, ":")
			panic(bailout{})
		}
		a.Value = p.parseExpr()
		c.Args = append(c.Args, a)
		if p.skipNewlines(); !p.at(COMMA) {
			break
		}
		p.next()
	}
	p.expect(RPAREN, "to close the argument list")
	c.Span = p.span(fn.Sp().Start)
	return c
}

func (p *parser) parseRecordLit(module, name string, start diag.Pos) *RecordLit {
	p.next() // {
	r := &RecordLit{Module: module, Name: name}
	for p.skipNewlines(); !p.at(RBRACE); p.skipNewlines() {
		if p.at(DOTDOT) {
			p.next()
			if r.Base != nil {
				p.errAt(p.peek().Span, "E117", "only one `..base` is allowed in a record literal")
			}
			r.Base = p.parseExpr()
		} else {
			fn := p.expectIdent("as field name in record literal")
			if !p.at(COLON) {
				p.errAt(p.peek().Span, "E117", "record literal fields are written `%s: value`", fn.Text).
					WithFix("write `"+fn.Text+": "+fn.Text+"`", diag.Span{File: p.file, Start: fn.Span.End, End: fn.Span.End}, ": "+fn.Text)
				panic(bailout{})
			}
			p.next()
			r.Fields = append(r.Fields, &FieldInit{Name: fn.Text, NameSpan: fn.Span, Value: p.parseExpr()})
		}
		if p.skipNewlines(); !p.at(COMMA) {
			break
		}
		p.next()
	}
	p.expect(RBRACE, "to close the record literal")
	r.Span = p.span(start)
	return r
}

func (p *parser) parsePrimary() Expr {
	t := p.peek()
	switch t.Kind {
	case INT:
		p.next()
		v, err := strconv.ParseInt(t.Text, 10, 64)
		if err != nil {
			p.errAt(t.Span, "E118", "integer literal `%s` does not fit in 64 bits", t.Text)
		}
		return &IntLit{Value: v, Text: t.Text, Span: t.Span}
	case FLOAT:
		p.next()
		v, _ := strconv.ParseFloat(t.Text, 64)
		return &FloatLit{Value: v, Text: t.Text, Span: t.Span}
	case STRING:
		p.next()
		return p.makeStr(t)
	case KW_TRUE, KW_FALSE:
		p.next()
		return &BoolLit{Value: t.Kind == KW_TRUE, Span: t.Span}
	case IDENT:
		p.next()
		if IsUpper(t.Text) && p.at(LBRACE) {
			return p.parseRecordLit("", t.Text, t.Span.Start)
		}
		switch t.Text {
		case "null", "nil", "undefined":
			p.errAt(t.Span, "E119", "Veld has no `%s`", t.Text).
				WithFix("use `None` (of type Option[T])", t.Span, "None")
			return &Ident{Name: "None", Span: t.Span}
		case "self", "this":
			p.errAt(t.Span, "E119", "Veld has no `%s`; functions take their data as explicit parameters", t.Text)
		case "lambda":
			p.errAt(t.Span, "E119", "lambdas are written `fn(x) => expr`")
		}
		return &Ident{Name: t.Text, Span: t.Span}
	case LPAREN:
		p.next()
		p.skipNewlines()
		x := p.parseExpr()
		p.skipNewlines()
		if p.at(COMMA) {
			p.errAt(p.peek().Span, "E119", "Veld has no tuples").Note("define a `record` to group values")
			panic(bailout{})
		}
		p.expect(RPAREN, "to close the parenthesis")
		return &ParenExpr{X: x, Span: p.span(t.Span.Start)}
	case LBRACK:
		p.next()
		l := &ListLit{}
		for p.skipNewlines(); !p.at(RBRACK); p.skipNewlines() {
			l.Elems = append(l.Elems, p.parseExpr())
			if p.skipNewlines(); !p.at(COMMA) {
				break
			}
			p.next()
		}
		p.expect(RBRACK, "to close the list")
		l.Span = p.span(t.Span.Start)
		return l
	case LBRACE:
		p.next()
		m := &MapLit{}
		for p.skipNewlines(); !p.at(RBRACE); p.skipNewlines() {
			k := p.parseExpr()
			p.expect(COLON, "between map key and value")
			v := p.parseExpr()
			m.Entries = append(m.Entries, &MapEntry{Key: k, Value: v})
			if p.skipNewlines(); !p.at(COMMA) {
				break
			}
			p.next()
		}
		p.expect(RBRACE, "to close the map literal")
		m.Span = p.span(t.Span.Start)
		return m
	case KW_FN:
		return p.parseLambda()
	case KW_IF:
		return p.parseIf()
	case KW_MATCH:
		return p.parseMatch()
	case KW_FOR:
		return p.parseFor()
	case KW_WHILE:
		return p.parseWhile()
	case KW_RETURN:
		p.next()
		r := &ReturnExpr{}
		switch p.peek().Kind {
		case NEWLINE, EOF, KW_END, RPAREN, RBRACK, RBRACE, COMMA, KW_ELSE, KW_CASE:
		default:
			r.X = p.parseExpr()
		}
		r.Span = p.span(t.Span.Start)
		return r
	case KW_BREAK:
		p.next()
		return &BreakExpr{Span: t.Span}
	case KW_CONTINUE:
		p.next()
		return &ContinueExpr{Span: t.Span}
	case HOLE:
		p.next()
		return &TodoExpr{Span: t.Span}
	}
	d := p.errAt(t.Span, "E104", "expected an expression, found %s", p.describe(t))
	if t.Kind == KW_END {
		d.Note("a block is closed here, but an expression was still expected")
	}
	panic(bailout{})
}

func (p *parser) makeStr(t Token) *StrLit {
	s := &StrLit{Raw: t.Raw, Span: t.Span}
	for _, part := range t.Parts {
		if part.IsExpr {
			s.Segs = append(s.Segs, StrSeg{Expr: parseExprAt(p.file, part.Src, part.Start, p.diags)})
		} else if part.Lit != "" || len(t.Parts) == 1 {
			s.Segs = append(s.Segs, StrSeg{Lit: part.Lit})
		}
	}
	return s
}

func (p *parser) parseLambda() Expr {
	start := p.next().Span.Start // fn
	if p.at(IDENT) {
		p.errAt(p.peek().Span, "E120", "named functions can only be declared at the top level").
			Note("use a lambda: `let f = fn(x) => x + 1`")
		panic(bailout{})
	}
	l := &LambdaExpr{Params: p.parseParams(false)}
	if p.at(ARROW) {
		p.next()
		l.Ret = p.parseType()
	}
	if p.at(FATARROW) {
		p.next()
		x := p.parseExpr()
		l.Short = true
		l.Body = &Block{Stmts: []Stmt{&ExprStmt{X: x, Span: x.Sp()}}, Span: x.Sp()}
		l.Span = p.span(start)
		return l
	}
	if !p.at(NEWLINE) {
		p.errAt(p.peek().Span, "E120", "expected `=>` or a new line after lambda parameters, found %s", p.describe(p.peek())).
			Note("write `fn(x) => expr` or a multi-line body closed with `end fn`")
		panic(bailout{})
	}
	p.skipNewlines()
	l.Body = p.parseBlock(KW_END)
	p.parseEnd(KW_FN, "fn", start)
	l.Span = p.span(start)
	return l
}

func (p *parser) condLine(what string) {
	if p.at(NEWLINE) {
		p.skipNewlines()
		return
	}
	t := p.peek()
	d := p.errAt(t.Span, "E105", "expected a new line after the `%s` condition, found %s", what, p.describe(t))
	if t.Kind == IDENT && (t.Text == "then" || t.Text == "do") || t.Kind == COLON || t.Kind == LBRACE {
		d.WithFix("remove it; the block starts on the next line", t.Span, "")
	}
	panic(bailout{})
}

func (p *parser) parseIf() Expr {
	start := p.next().Span.Start
	e := &IfExpr{}
	cond := p.parseExpr()
	p.condLine("if")
	body := p.parseBlock(KW_ELSE, KW_END)
	e.Branches = append(e.Branches, &IfBranch{Cond: cond, Body: body})
	for p.at(KW_ELSE) {
		p.next()
		if p.at(KW_IF) {
			p.next()
			c := p.parseExpr()
			p.condLine("else if")
			b := p.parseBlock(KW_ELSE, KW_END)
			e.Branches = append(e.Branches, &IfBranch{Cond: c, Body: b})
			continue
		}
		if !p.at(NEWLINE) {
			p.errAt(p.peek().Span, "E105", "expected a new line after `else`").
				Note("the else-block starts on the next line")
			panic(bailout{})
		}
		p.skipNewlines()
		e.Else = p.parseBlock(KW_END, KW_ELSE)
		if p.at(KW_ELSE) {
			p.errAt(p.peek().Span, "E121", "`else` must be the last branch of an `if`")
			panic(bailout{})
		}
		break
	}
	p.parseEnd(KW_IF, "if", start)
	e.Span = p.span(start)
	return e
}

func (p *parser) parseMatch() Expr {
	start := p.next().Span.Start
	m := &MatchExpr{X: p.parseExpr()}
	p.condLine("match")
	for p.at(KW_CASE) {
		as := p.peek().Span.Start
		lead := p.leading(as.Line)
		p.next()
		arm := &Arm{Pattern: p.parsePattern()}
		arm.Leading = lead
		if p.at(KW_IF) {
			p.next()
			arm.Guard = p.parseExpr()
		}
		if p.at(FATARROW) {
			p.next()
			arm.Short = true
			if p.at(KW_SET) || p.at(KW_EXPECT) {
				st := p.parseSimpleStmt()
				arm.Body = &Block{Stmts: []Stmt{st}, Span: st.Sp()}
			} else {
				x := p.parseExpr()
				arm.Body = &Block{Stmts: []Stmt{&ExprStmt{X: x, Span: x.Sp()}}, Span: x.Sp()}
			}
			arm.Trailing = p.trailing(p.prevEnd().Line)
			if !p.at(NEWLINE) {
				p.errAt(p.peek().Span, "E105", "expected end of line after match arm, found %s", p.describe(p.peek()))
				panic(bailout{})
			}
			p.skipNewlines()
		} else if p.at(NEWLINE) {
			arm.Trailing = p.trailing(p.prevEnd().Line)
			p.skipNewlines()
			arm.Body = p.parseBlock(KW_CASE, KW_END)
		} else {
			t := p.peek()
			d := p.errAt(t.Span, "E122", "expected `=>` or a new line after the case pattern, found %s", p.describe(t))
			if t.Kind == COLON || t.Kind == ARROW {
				d.WithFix("use `=>`", t.Span, "=>")
			}
			panic(bailout{})
		}
		arm.Span = p.span(as)
		m.Arms = append(m.Arms, arm)
	}
	if len(m.Arms) == 0 {
		p.errAt(p.peek().Span, "E122", "`match` needs at least one `case` arm, found %s", p.describe(p.peek()))
		panic(bailout{})
	}
	m.EndComments = p.leading(p.peek().Span.Start.Line)
	m.EndPos = p.peek().Span.Start
	p.parseEnd(KW_MATCH, "match", start)
	m.Span = p.span(start)
	return m
}

func (p *parser) parseFor() Expr {
	start := p.next().Span.Start
	v := p.expectIdent("as loop variable")
	if p.at(COMMA) {
		p.errAt(p.peek().Span, "E123", "`for` binds a single variable").
			Note("to iterate with indices: for i in list.range(0, list.len(xs))")
		panic(bailout{})
	}
	p.expect(KW_IN, "in for loop")
	f := &ForExpr{Var: v.Text, VarSpan: v.Span, Iter: p.parseExpr()}
	p.condLine("for")
	f.Body = p.parseBlock(KW_END)
	p.parseEnd(KW_FOR, "for", start)
	f.Span = p.span(start)
	return f
}

func (p *parser) parseWhile() Expr {
	start := p.next().Span.Start
	w := &WhileExpr{Cond: p.parseExpr()}
	p.condLine("while")
	w.Body = p.parseBlock(KW_END)
	p.parseEnd(KW_WHILE, "while", start)
	w.Span = p.span(start)
	return w
}

// ---- patterns ----

// parsePattern parses a pattern, including `p1 | p2` alternatives.
func (p *parser) parsePattern() Pattern {
	pat := p.parsePatternAtom()
	if !p.at(BAR) {
		return pat
	}
	op := &OrPat{Alts: []Pattern{pat}}
	for p.at(BAR) {
		p.next()
		op.Alts = append(op.Alts, p.parsePatternAtom())
	}
	op.Span = diag.Join(pat.Sp(), op.Alts[len(op.Alts)-1].Sp())
	return op
}

func (p *parser) parsePatternAtom() Pattern {
	t := p.peek()
	switch t.Kind {
	case IDENT:
		p.next()
		if t.Text == "_" {
			return &WildPat{Span: t.Span}
		}
		if !IsUpper(t.Text) {
			if p.at(DOT) && p.peekN(1).Kind == IDENT && IsUpper(p.peekN(1).Text) {
				p.next()
				n := p.next()
				return p.ctorPatRest(t.Text, n.Text, t.Span.Start)
			}
			return &BindPat{Name: t.Text, Span: t.Span}
		}
		return p.ctorPatRest("", t.Text, t.Span.Start)
	case INT:
		p.next()
		v, _ := strconv.ParseInt(t.Text, 10, 64)
		return &LitPat{Value: &IntLit{Value: v, Text: t.Text, Span: t.Span}, Span: t.Span}
	case MINUS:
		p.next()
		n := p.expect(INT, "after `-` in pattern")
		v, _ := strconv.ParseInt(n.Text, 10, 64)
		sp := diag.Join(t.Span, n.Span)
		return &LitPat{Value: &IntLit{Value: -v, Text: "-" + n.Text, Span: sp}, Span: sp}
	case FLOAT:
		p.errAt(t.Span, "E124", "float literals cannot be used as patterns").Note("use a guard: `case x if x == 1.5 =>`")
		panic(bailout{})
	case STRING:
		p.next()
		s := p.makeStr(t)
		for _, seg := range s.Segs {
			if seg.Expr != nil {
				p.errAt(t.Span, "E124", "string patterns cannot contain interpolation")
			}
		}
		return &LitPat{Value: s, Span: t.Span}
	case KW_TRUE, KW_FALSE:
		p.next()
		return &LitPat{Value: &BoolLit{Value: t.Kind == KW_TRUE, Span: t.Span}, Span: t.Span}
	case LBRACK:
		p.next()
		lp := &ListPat{}
		for p.skipNewlines(); !p.at(RBRACK); p.skipNewlines() {
			if p.at(DOTDOT) {
				p.next()
				lp.HasRest = true
				if p.at(IDENT) {
					lp.Rest = p.next().Text
				}
				for p.skipNewlines(); p.at(COMMA); p.skipNewlines() {
					p.next()
					p.skipNewlines()
					if p.at(RBRACK) {
						break
					}
					if p.at(DOTDOT) {
						p.errAt(p.peek().Span, "E124", "a list pattern can contain only one `..rest`")
						panic(bailout{})
					}
					lp.Suffix = append(lp.Suffix, p.parsePattern())
				}
				break
			}
			lp.Elems = append(lp.Elems, p.parsePattern())
			if p.skipNewlines(); !p.at(COMMA) {
				break
			}
			p.next()
		}
		p.expect(RBRACK, "to close the list pattern")
		lp.Span = p.span(t.Span.Start)
		return lp
	}
	p.errAt(t.Span, "E124", "expected a pattern, found %s", p.describe(t))
	panic(bailout{})
}

func (p *parser) ctorPatRest(module, name string, start diag.Pos) Pattern {
	cp := &CtorPat{Module: module, Name: name}
	if p.at(LPAREN) {
		p.next()
		cp.HasParens = true
		for p.skipNewlines(); !p.at(RPAREN); p.skipNewlines() {
			cp.Args = append(cp.Args, p.parsePattern())
			if p.skipNewlines(); !p.at(COMMA) {
				break
			}
			p.next()
		}
		p.expect(RPAREN, "to close the constructor pattern")
	}
	if p.at(LBRACE) {
		p.errAt(p.peek().Span, "E124", "record patterns are not supported; bind the value and access fields with `.`")
		panic(bailout{})
	}
	cp.Span = p.span(start)
	return cp
}
