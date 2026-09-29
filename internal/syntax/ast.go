package syntax

import "github.com/ashraf82de/veld/internal/diag"

// Meta carries comments attached to a declaration, statement, arm or field.
type Meta struct {
	Blank    bool     `json:"-"` // preceded by a blank line in the source
	Leading  []string `json:"leading,omitempty"`
	Trailing string   `json:"trailing,omitempty"`
}

type File struct {
	Path        string   `json:"path"`
	Uses        []*Use   `json:"uses"`
	Decls       []Decl   `json:"decls"`
	EndComments []string `json:"end_comments,omitempty"`
	Source      string   `json:"-"`
}

type Use struct {
	Meta
	Path []string  `json:"path"`
	Span diag.Span `json:"span"`
}

// Name returns the local name a use binds (the last path segment).
func (u *Use) Name() string { return u.Path[len(u.Path)-1] }

type Decl interface {
	Node
	DeclName() string
	isDecl()
}

type Node interface {
	Sp() diag.Span
}

type Param struct {
	Name string    `json:"name"`
	Type TypeExpr  `json:"type,omitempty"`
	Span diag.Span `json:"span"`
}

type FnDecl struct {
	Meta
	Doc      []string  `json:"doc,omitempty"`
	Pub      bool      `json:"pub"`
	Extern   bool      `json:"extern"`
	Name     string    `json:"name"`
	NameSpan diag.Span `json:"name_span"`
	TParams  []string  `json:"type_params,omitempty"`
	Params   []*Param  `json:"params"`
	Ret      TypeExpr  `json:"ret"`
	Effects  []string  `json:"effects,omitempty"`
	HeadEnd  diag.Pos  `json:"-"` // position just after the signature (for fix-its)
	Requires []Expr    `json:"requires,omitempty"`
	Ensures  []Expr    `json:"ensures,omitempty"`
	Body     *Block    `json:"body,omitempty"`
	Span     diag.Span `json:"span"`
}

type Field struct {
	Meta
	Name string    `json:"name"`
	Type TypeExpr  `json:"type"`
	Span diag.Span `json:"span"`
}

type Variant struct {
	Meta
	Name   string    `json:"name"`
	Fields []*Field  `json:"fields,omitempty"`
	Span   diag.Span `json:"span"`
}

type TypeDecl struct {
	Meta
	Doc         []string   `json:"doc,omitempty"`
	Pub         bool       `json:"pub"`
	Name        string     `json:"name"`
	TParams     []string   `json:"type_params,omitempty"`
	Variants    []*Variant `json:"variants"`
	Span        diag.Span  `json:"span"`
	EndComments []string   `json:"end_comments,omitempty"`
}

type RecordDecl struct {
	Meta
	Doc         []string  `json:"doc,omitempty"`
	Pub         bool      `json:"pub"`
	Name        string    `json:"name"`
	TParams     []string  `json:"type_params,omitempty"`
	Fields      []*Field  `json:"fields"`
	Span        diag.Span `json:"span"`
	EndComments []string  `json:"end_comments,omitempty"`
}

type TestDecl struct {
	Meta
	Name string    `json:"name"`
	Body *Block    `json:"body"`
	Span diag.Span `json:"span"`
}

func (d *FnDecl) Sp() diag.Span        { return d.Span }
func (d *TypeDecl) Sp() diag.Span      { return d.Span }
func (d *RecordDecl) Sp() diag.Span    { return d.Span }
func (d *TestDecl) Sp() diag.Span      { return d.Span }
func (d *FnDecl) DeclName() string     { return d.Name }
func (d *TypeDecl) DeclName() string   { return d.Name }
func (d *RecordDecl) DeclName() string { return d.Name }
func (d *TestDecl) DeclName() string   { return d.Name }
func (*FnDecl) isDecl()                {}
func (*TypeDecl) isDecl()              {}
func (*RecordDecl) isDecl()            {}
func (*TestDecl) isDecl()              {}

// ---- types ----

type TypeExpr interface {
	Node
	isType()
}

type NamedType struct {
	Module string     `json:"module,omitempty"`
	Name   string     `json:"name"`
	Args   []TypeExpr `json:"args,omitempty"`
	Span   diag.Span  `json:"span"`
}

type FnType struct {
	Params  []TypeExpr `json:"params"`
	Ret     TypeExpr   `json:"ret"`
	Effects []string   `json:"effects,omitempty"`
	Span    diag.Span  `json:"span"`
}

func (t *NamedType) Sp() diag.Span { return t.Span }
func (t *FnType) Sp() diag.Span    { return t.Span }
func (*NamedType) isType()         {}
func (*FnType) isType()            {}

// ---- statements ----

type Block struct {
	Stmts       []Stmt    `json:"stmts"`
	EndComments []string  `json:"end_comments,omitempty"`
	Span        diag.Span `json:"span"`
}

type Stmt interface {
	Node
	M() *Meta
	isStmt()
}

type LetStmt struct {
	Meta
	Mutable  bool      `json:"mutable"`
	Name     string    `json:"name"`
	NameSpan diag.Span `json:"name_span"`
	Type     TypeExpr  `json:"type,omitempty"`
	Value    Expr      `json:"value"`
	Span     diag.Span `json:"span"`
}

type SetStmt struct {
	Meta
	Name     string    `json:"name"`
	NameSpan diag.Span `json:"name_span"`
	Value    Expr      `json:"value"`
	Span     diag.Span `json:"span"`
}

type ExprStmt struct {
	Meta
	X    Expr      `json:"x"`
	Span diag.Span `json:"span"`
}

type ExpectStmt struct {
	Meta
	X    Expr      `json:"x"`
	Span diag.Span `json:"span"`
}

func (s *LetStmt) Sp() diag.Span    { return s.Span }
func (s *SetStmt) Sp() diag.Span    { return s.Span }
func (s *ExprStmt) Sp() diag.Span   { return s.Span }
func (s *ExpectStmt) Sp() diag.Span { return s.Span }
func (s *LetStmt) M() *Meta         { return &s.Meta }
func (s *SetStmt) M() *Meta         { return &s.Meta }
func (s *ExprStmt) M() *Meta        { return &s.Meta }
func (s *ExpectStmt) M() *Meta      { return &s.Meta }
func (*LetStmt) isStmt()            {}
func (*SetStmt) isStmt()            {}
func (*ExprStmt) isStmt()           {}
func (*ExpectStmt) isStmt()         {}

// ---- expressions ----

type Expr interface {
	Node
	isExpr()
}

type IntLit struct {
	Value int64     `json:"value"`
	Text  string    `json:"text"`
	Span  diag.Span `json:"span"`
}

type FloatLit struct {
	Value float64   `json:"value"`
	Text  string    `json:"text"`
	Span  diag.Span `json:"span"`
}

// StrSeg is either literal text or an interpolated expression.
type StrSeg struct {
	Lit  string `json:"lit,omitempty"`
	Expr Expr   `json:"expr,omitempty"`
}

type StrLit struct {
	Segs []StrSeg  `json:"segs"`
	Raw  bool      `json:"raw,omitempty"`
	Span diag.Span `json:"span"`
}

type BoolLit struct {
	Value bool      `json:"value"`
	Span  diag.Span `json:"span"`
}

type Ident struct {
	Name string    `json:"name"`
	Span diag.Span `json:"span"`
}

type FieldExpr struct {
	X        Expr      `json:"x"`
	Name     string    `json:"name"`
	NameSpan diag.Span `json:"name_span"`
	Span     diag.Span `json:"span"`
}

type Arg struct {
	Name     string    `json:"name,omitempty"`
	NameSpan diag.Span `json:"name_span,omitempty"`
	Value    Expr      `json:"value"`
}

type CallExpr struct {
	Fn     Expr      `json:"fn"`
	Args   []*Arg    `json:"args"`
	Span   diag.Span `json:"span"`
	LParen diag.Pos  `json:"-"`
	// Synthetic marks the constructor call the parser builds for the values of
	// a multi-value `match a, b`; it is not in the source.
	Synthetic bool `json:"-"`
}

// PipeExpr is `L |> R` where R is a call; L becomes R's first argument.
type PipeExpr struct {
	L    Expr      `json:"l"`
	R    *CallExpr `json:"r"`
	Span diag.Span `json:"span"`
}

type BinaryExpr struct {
	Op     Kind      `json:"op"`
	L      Expr      `json:"l"`
	R      Expr      `json:"r"`
	OpSpan diag.Span `json:"op_span"`
	Span   diag.Span `json:"span"`
}

type UnaryExpr struct {
	Op   Kind      `json:"op"`
	X    Expr      `json:"x"`
	Span diag.Span `json:"span"`
}

type ParenExpr struct {
	X    Expr      `json:"x"`
	Span diag.Span `json:"span"`
}

type ListLit struct {
	Elems []Expr    `json:"elems"`
	Span  diag.Span `json:"span"`
}

type MapEntry struct {
	Key   Expr `json:"key"`
	Value Expr `json:"value"`
}

type MapLit struct {
	Entries []*MapEntry `json:"entries"`
	Span    diag.Span   `json:"span"`
}

type FieldInit struct {
	Name     string    `json:"name"`
	NameSpan diag.Span `json:"name_span"`
	Value    Expr      `json:"value"`
}

type RecordLit struct {
	Module string       `json:"module,omitempty"`
	Name   string       `json:"name"`
	Base   Expr         `json:"base,omitempty"`
	Fields []*FieldInit `json:"fields"`
	Span   diag.Span    `json:"span"`
}

type LambdaExpr struct {
	Params []*Param  `json:"params"`
	Ret    TypeExpr  `json:"ret,omitempty"`
	Body   *Block    `json:"body"`
	Short  bool      `json:"short"` // fn(x) => expr form
	Span   diag.Span `json:"span"`
}

type IfBranch struct {
	Cond Expr   `json:"cond"`
	Body *Block `json:"body"`
}

type IfExpr struct {
	Branches []*IfBranch `json:"branches"`
	Else     *Block      `json:"else,omitempty"`
	Span     diag.Span   `json:"span"`
}

type Arm struct {
	Meta
	Pattern Pattern   `json:"pattern"`
	Guard   Expr      `json:"guard,omitempty"`
	Body    *Block    `json:"body"`
	Short   bool      `json:"short"` // case p => expr
	Span    diag.Span `json:"span"`
}

type MatchExpr struct {
	EndPos diag.Pos `json:"-"`
	X      Expr     `json:"x"`
	// Values lists the scrutinees of a multi-value match (`match a, b`); X is
	// then a synthetic TupleN constructor call over them. Nil otherwise.
	Values      []Expr    `json:"values,omitempty"`
	Arms        []*Arm    `json:"arms"`
	Span        diag.Span `json:"span"`
	EndComments []string  `json:"end_comments,omitempty"`
}

type ForExpr struct {
	Var     string    `json:"var"`
	VarSpan diag.Span `json:"var_span"`
	Iter    Expr      `json:"iter"`
	Body    *Block    `json:"body"`
	Span    diag.Span `json:"span"`
}

type WhileExpr struct {
	Cond Expr      `json:"cond"`
	Body *Block    `json:"body"`
	Span diag.Span `json:"span"`
}

type ReturnExpr struct {
	X    Expr      `json:"x,omitempty"`
	Span diag.Span `json:"span"`
}

type BreakExpr struct {
	Span diag.Span `json:"span"`
}

type ContinueExpr struct {
	Span diag.Span `json:"span"`
}

type TodoExpr struct {
	Span diag.Span `json:"span"`
}

// TryExpr is postfix `?`: unwraps Ok/Some or returns Err/None early.
type TryExpr struct {
	X    Expr      `json:"x"`
	Span diag.Span `json:"span"`
}

func (e *IntLit) Sp() diag.Span       { return e.Span }
func (e *FloatLit) Sp() diag.Span     { return e.Span }
func (e *StrLit) Sp() diag.Span       { return e.Span }
func (e *BoolLit) Sp() diag.Span      { return e.Span }
func (e *Ident) Sp() diag.Span        { return e.Span }
func (e *FieldExpr) Sp() diag.Span    { return e.Span }
func (e *CallExpr) Sp() diag.Span     { return e.Span }
func (e *PipeExpr) Sp() diag.Span     { return e.Span }
func (e *BinaryExpr) Sp() diag.Span   { return e.Span }
func (e *UnaryExpr) Sp() diag.Span    { return e.Span }
func (e *ParenExpr) Sp() diag.Span    { return e.Span }
func (e *ListLit) Sp() diag.Span      { return e.Span }
func (e *MapLit) Sp() diag.Span       { return e.Span }
func (e *RecordLit) Sp() diag.Span    { return e.Span }
func (e *LambdaExpr) Sp() diag.Span   { return e.Span }
func (e *IfExpr) Sp() diag.Span       { return e.Span }
func (e *MatchExpr) Sp() diag.Span    { return e.Span }
func (e *ForExpr) Sp() diag.Span      { return e.Span }
func (e *WhileExpr) Sp() diag.Span    { return e.Span }
func (e *ReturnExpr) Sp() diag.Span   { return e.Span }
func (e *BreakExpr) Sp() diag.Span    { return e.Span }
func (e *ContinueExpr) Sp() diag.Span { return e.Span }
func (e *TodoExpr) Sp() diag.Span     { return e.Span }
func (e *TryExpr) Sp() diag.Span      { return e.Span }

func (*IntLit) isExpr()       {}
func (*FloatLit) isExpr()     {}
func (*StrLit) isExpr()       {}
func (*BoolLit) isExpr()      {}
func (*Ident) isExpr()        {}
func (*FieldExpr) isExpr()    {}
func (*CallExpr) isExpr()     {}
func (*PipeExpr) isExpr()     {}
func (*BinaryExpr) isExpr()   {}
func (*UnaryExpr) isExpr()    {}
func (*ParenExpr) isExpr()    {}
func (*ListLit) isExpr()      {}
func (*MapLit) isExpr()       {}
func (*RecordLit) isExpr()    {}
func (*LambdaExpr) isExpr()   {}
func (*IfExpr) isExpr()       {}
func (*MatchExpr) isExpr()    {}
func (*ForExpr) isExpr()      {}
func (*WhileExpr) isExpr()    {}
func (*ReturnExpr) isExpr()   {}
func (*BreakExpr) isExpr()    {}
func (*ContinueExpr) isExpr() {}
func (*TodoExpr) isExpr()     {}
func (*TryExpr) isExpr()      {}

// ---- patterns ----

type Pattern interface {
	Node
	isPattern()
}

type WildPat struct {
	Span diag.Span `json:"span"`
}

type BindPat struct {
	Name string    `json:"name"`
	Span diag.Span `json:"span"`
}

type LitPat struct {
	Value Expr      `json:"value"` // IntLit, StrLit, BoolLit (IntLit may be negative)
	Span  diag.Span `json:"span"`
}

type CtorPat struct {
	// Tuple marks the TupleN pattern of a multi-value `case p, q`.
	Tuple     bool      `json:"-"`
	Module    string    `json:"module,omitempty"`
	Name      string    `json:"name"`
	Args      []Pattern `json:"args,omitempty"`
	HasParens bool      `json:"-"`
	Span      diag.Span `json:"span"`
}

type ListPat struct {
	Elems   []Pattern `json:"elems"`
	HasRest bool      `json:"has_rest"`
	Rest    string    `json:"rest,omitempty"`   // "" or "_" means ignore
	Suffix  []Pattern `json:"suffix,omitempty"` // patterns after ..rest
	Span    diag.Span `json:"span"`
}

// OrPat matches if any alternative matches. Alternatives may not bind names.
type OrPat struct {
	Alts []Pattern `json:"alts"`
	Span diag.Span `json:"span"`
}

func (p *OrPat) Sp() diag.Span { return p.Span }
func (*OrPat) isPattern()      {}

func (p *WildPat) Sp() diag.Span { return p.Span }
func (p *BindPat) Sp() diag.Span { return p.Span }
func (p *LitPat) Sp() diag.Span  { return p.Span }
func (p *CtorPat) Sp() diag.Span { return p.Span }
func (p *ListPat) Sp() diag.Span { return p.Span }
func (*WildPat) isPattern()      {}
func (*BindPat) isPattern()      {}
func (*LitPat) isPattern()       {}
func (*CtorPat) isPattern()      {}
func (*ListPat) isPattern()      {}

// IsUpper reports whether a name is a type/constructor name.
func IsUpper(name string) bool { return name != "" && name[0] >= 'A' && name[0] <= 'Z' }

// FieldPat is one `name` or `name: pattern` entry of a record pattern. For the
// shorthand `name`, Pat is a BindPat of the same name and Shorthand is set.
type FieldPat struct {
	Name      string    `json:"name"`
	Pat       Pattern   `json:"pattern"`
	Shorthand bool      `json:"shorthand,omitempty"`
	Span      diag.Span `json:"span"`
}

// RecordPat matches a record: `User{name, age: 3, ..}`.
type RecordPat struct {
	Module  string      `json:"module,omitempty"`
	Name    string      `json:"name"`
	Fields  []*FieldPat `json:"fields"`
	HasRest bool        `json:"has_rest,omitempty"` // trailing `..`
	Span    diag.Span   `json:"span"`
}

func (p *RecordPat) Sp() diag.Span { return p.Span }
func (*RecordPat) isPattern()      {}
