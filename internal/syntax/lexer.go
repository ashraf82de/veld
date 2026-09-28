package syntax

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ashraf82de/veld/internal/diag"
)

type Kind int

const (
	EOF Kind = iota
	NEWLINE
	IDENT
	INT
	FLOAT
	STRING
	// punctuation
	LPAREN
	RPAREN
	LBRACK
	RBRACK
	LBRACE
	RBRACE
	COMMA
	COLON
	DOT
	DOTDOT
	ARROW    // ->
	FATARROW // =>
	PIPE     // |>
	ASSIGN   // =
	EQ       // ==
	NE       // !=
	LT
	LE
	GT
	GE
	PLUS
	MINUS
	STAR
	SLASH
	PERCENT
	QUESTION
	HOLE // ???
	BAR  // | (or-patterns)
	// keywords
	kwStart
	KW_FN
	KW_TYPE
	KW_RECORD
	KW_END
	KW_LET
	KW_VAR
	KW_SET
	KW_IF
	KW_ELSE
	KW_MATCH
	KW_CASE
	KW_RETURN
	KW_FOR
	KW_IN
	KW_WHILE
	KW_BREAK
	KW_CONTINUE
	KW_TEST
	KW_EXPECT
	KW_REQUIRES
	KW_ENSURES
	KW_USES
	KW_AND
	KW_OR
	KW_NOT
	KW_TRUE
	KW_FALSE
	KW_USE
	KW_PUB
	KW_EXTERN
	kwEnd
)

var kindNames = map[Kind]string{
	EOF: "end of file", NEWLINE: "newline", IDENT: "identifier", INT: "integer", FLOAT: "float", STRING: "string",
	LPAREN: "(", RPAREN: ")", LBRACK: "[", RBRACK: "]", LBRACE: "{", RBRACE: "}", COMMA: ",", COLON: ":",
	DOT: ".", DOTDOT: "..", ARROW: "->", FATARROW: "=>", PIPE: "|>", ASSIGN: "=", EQ: "==", NE: "!=",
	LT: "<", LE: "<=", GT: ">", GE: ">=", PLUS: "+", MINUS: "-", STAR: "*", SLASH: "/", PERCENT: "%",
	QUESTION: "?", HOLE: "???", BAR: "|",
}

// Keywords maps keyword spelling to kind.
var Keywords = map[string]Kind{
	"fn": KW_FN, "type": KW_TYPE, "record": KW_RECORD, "end": KW_END, "let": KW_LET, "var": KW_VAR,
	"set": KW_SET, "if": KW_IF, "else": KW_ELSE, "match": KW_MATCH, "case": KW_CASE, "return": KW_RETURN,
	"for": KW_FOR, "in": KW_IN, "while": KW_WHILE, "break": KW_BREAK, "continue": KW_CONTINUE,
	"test": KW_TEST, "expect": KW_EXPECT, "requires": KW_REQUIRES, "ensures": KW_ENSURES, "uses": KW_USES,
	"and": KW_AND, "or": KW_OR, "not": KW_NOT, "true": KW_TRUE, "false": KW_FALSE,
	"use": KW_USE, "pub": KW_PUB, "extern": KW_EXTERN,
}

func init() {
	for k, v := range Keywords {
		kindNames[v] = k
	}
}

func (k Kind) String() string {
	if s, ok := kindNames[k]; ok {
		return s
	}
	return fmt.Sprintf("token(%d)", int(k))
}

func (k Kind) IsKeyword() bool { return k > kwStart && k < kwEnd }

// StrPart is one piece of a string literal: either literal text or the
// source of an interpolated expression.
type StrPart struct {
	Lit    string
	IsExpr bool
	Src    string   // expression source when IsExpr
	Start  diag.Pos // position of Src[0]
}

type Token struct {
	Kind  Kind
	Text  string
	Span  diag.Span
	Parts []StrPart // for STRING
	Raw   bool      // triple-quoted string
}

// Comment is a source comment. Doc comments start with "##".
type Comment struct {
	Text string // including the leading #
	Line int
	Own  bool // true if the comment is alone on its line
}

type lexer struct {
	file         string
	src          string
	off          int
	line         int
	col          int
	toks         []Token
	comments     []Comment
	diags        *diag.List
	depth        int // bracket nesting; newlines are ignored inside brackets
	lineHasToken bool
	baseOff      int
}

// Lex tokenizes src. Newlines are significant statement terminators except
// inside brackets, after a token that cannot end an expression, or before a
// line that starts with "|>" or ".".
func Lex(file, src string, diags *diag.List) ([]Token, []Comment) {
	return lexAt(file, src, diag.Pos{Line: 1, Col: 1}, diags)
}

func lexAt(file, src string, start diag.Pos, diags *diag.List) ([]Token, []Comment) {
	lx := &lexer{file: file, src: src, line: start.Line, col: start.Col, diags: diags}
	lx.baseOff = start.Offset
	lx.run()
	return lx.toks, lx.comments
}

func (lx *lexer) pos() diag.Pos {
	return diag.Pos{Line: lx.line, Col: lx.col, Offset: lx.off + lx.baseOff}
}

func (lx *lexer) peekc(n int) byte {
	if lx.off+n < len(lx.src) {
		return lx.src[lx.off+n]
	}
	return 0
}

func (lx *lexer) advance() rune {
	r, w := utf8.DecodeRuneInString(lx.src[lx.off:])
	lx.off += w
	if r == '\n' {
		lx.line++
		lx.col = 1
	} else {
		lx.col++
	}
	return r
}

func (lx *lexer) emit(k Kind, text string, start diag.Pos) *Token {
	lx.toks = append(lx.toks, Token{Kind: k, Text: text, Span: diag.Span{File: lx.file, Start: start, End: lx.pos()}})
	lx.lineHasToken = true
	return &lx.toks[len(lx.toks)-1]
}

func (lx *lexer) errf(start diag.Pos, format string, args ...any) *diag.Diagnostic {
	return lx.diags.Errorf("E101", diag.Span{File: lx.file, Start: start, End: lx.pos()}, format, args...)
}

// continuationEnders: tokens after which a newline does not end a statement.
func continues(k Kind) bool {
	switch k {
	case COMMA, ASSIGN, FATARROW, ARROW, PIPE, EQ, NE, LT, LE, GT, GE, PLUS, MINUS, STAR, SLASH, PERCENT,
		KW_AND, KW_OR, KW_NOT, LPAREN, LBRACK, LBRACE, DOT, COLON:
		return true
	}
	return false
}

func (lx *lexer) newline(start diag.Pos) {
	if len(lx.toks) == 0 {
		return
	}
	last := lx.toks[len(lx.toks)-1].Kind
	if last == NEWLINE || continues(last) {
		return
	}
	// Look ahead: a following line starting with |> or . continues the expression.
	i := lx.off
	for i < len(lx.src) {
		c := lx.src[i]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			i++
			continue
		}
		if c == '#' {
			for i < len(lx.src) && lx.src[i] != '\n' {
				i++
			}
			continue
		}
		break
	}
	if strings.HasPrefix(lx.src[i:], "|>") || (strings.HasPrefix(lx.src[i:], ".") && !strings.HasPrefix(lx.src[i:], "..")) {
		return
	}
	lx.toks = append(lx.toks, Token{Kind: NEWLINE, Text: "\n", Span: diag.Span{File: lx.file, Start: start, End: start}})
}

func (lx *lexer) run() {
	for lx.off < len(lx.src) {
		c := lx.src[lx.off]
		start := lx.pos()
		switch {
		case c == '\n':
			lx.newline(start)
			lx.advance()
			lx.lineHasToken = false
		case c == ' ' || c == '\t' || c == '\r':
			lx.advance()
		case c == '#':
			own := !lx.lineHasToken
			b := lx.off
			for lx.off < len(lx.src) && lx.src[lx.off] != '\n' {
				lx.advance()
			}
			lx.comments = append(lx.comments, Comment{Text: strings.TrimRight(lx.src[b:lx.off], " \t\r"), Line: start.Line, Own: own})
		case isIdentStart(c):
			b := lx.off
			for lx.off < len(lx.src) && isIdentChar(lx.src[lx.off]) {
				lx.advance()
			}
			word := lx.src[b:lx.off]
			if k, ok := Keywords[word]; ok {
				lx.emit(k, word, start)
			} else {
				lx.emit(IDENT, word, start)
			}
		case c >= '0' && c <= '9':
			lx.number(start)
		case c == '"':
			if strings.HasPrefix(lx.src[lx.off:], `"""`) {
				lx.rawString(start)
			} else {
				lx.str(start)
			}
		default:
			lx.punct(start)
		}
	}
	end := lx.pos()
	lx.newline(end)
	lx.toks = append(lx.toks, Token{Kind: EOF, Span: diag.Span{File: lx.file, Start: end, End: end}})
}

func isIdentStart(c byte) bool { return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isIdentChar(c byte) bool  { return isIdentStart(c) || (c >= '0' && c <= '9') }

func (lx *lexer) number(start diag.Pos) {
	b := lx.off
	isFloat := false
	digits := func() {
		for lx.off < len(lx.src) && (lx.src[lx.off] >= '0' && lx.src[lx.off] <= '9' || lx.src[lx.off] == '_') {
			lx.advance()
		}
	}
	digits()
	if lx.peekc(0) == '.' && lx.peekc(1) >= '0' && lx.peekc(1) <= '9' {
		isFloat = true
		lx.advance()
		digits()
	}
	if c := lx.peekc(0); c == 'e' || c == 'E' {
		n := 1
		if lx.peekc(1) == '+' || lx.peekc(1) == '-' {
			n = 2
		}
		if d := lx.peekc(n); d >= '0' && d <= '9' {
			isFloat = true
			for i := 0; i < n; i++ {
				lx.advance()
			}
			digits()
		}
	}
	text := strings.ReplaceAll(lx.src[b:lx.off], "_", "")
	if lx.off < len(lx.src) && isIdentStart(lx.src[lx.off]) {
		lx.errf(start, "invalid number literal")
	}
	if isFloat {
		lx.emit(FLOAT, text, start)
	} else {
		lx.emit(INT, text, start)
	}
}

func (lx *lexer) str(start diag.Pos) {
	lx.advance() // opening quote
	var parts []StrPart
	var lit strings.Builder
	for {
		if lx.off >= len(lx.src) || lx.src[lx.off] == '\n' {
			lx.errf(start, "unterminated string literal").WithFix("close the string", diag.Span{File: lx.file, Start: lx.pos(), End: lx.pos()}, `"`)
			break
		}
		c := lx.src[lx.off]
		if c == '"' {
			lx.advance()
			break
		}
		if c == '\\' {
			escStart := lx.pos()
			lx.advance()
			if lx.off >= len(lx.src) {
				continue
			}
			e := lx.advance()
			switch e {
			case 'n':
				lit.WriteByte('\n')
			case 't':
				lit.WriteByte('\t')
			case 'r':
				lit.WriteByte('\r')
			case '\\':
				lit.WriteByte('\\')
			case '"':
				lit.WriteByte('"')
			case '$':
				lit.WriteByte('$')
			case '0':
				lit.WriteByte(0)
			default:
				lx.diags.Errorf("E102", diag.Span{File: lx.file, Start: escStart, End: lx.pos()}, "unknown escape sequence \\%c", e).
					Note(`valid escapes: \n \t \r \\ \" \$ \0`)
			}
			continue
		}
		if c == '$' && lx.peekc(1) == '{' {
			lx.advance()
			if lit.Len() > 0 || len(parts) == 0 {
				parts = append(parts, StrPart{Lit: lit.String()})
				lit.Reset()
			}
			lx.advance()
			exprStart := lx.pos()
			b := lx.off
			depth := 0
			for lx.off < len(lx.src) && lx.src[lx.off] != '\n' {
				ch := lx.src[lx.off]
				if ch == '{' {
					depth++
				} else if ch == '}' {
					if depth == 0 {
						break
					}
					depth--
				} else if ch == '"' {
					lx.skipNestedString()
					continue
				}
				lx.advance()
			}
			if lx.off >= len(lx.src) || lx.src[lx.off] != '}' {
				lx.errf(exprStart, "unterminated interpolation; expected }")
				continue
			}
			src := lx.src[b:lx.off]
			lx.advance()
			if strings.TrimSpace(src) == "" {
				lx.diags.Errorf("E103", diag.Span{File: lx.file, Start: exprStart, End: lx.pos()}, "empty interpolation ${}").
					Note(`write \$ for a literal dollar sign`)
				continue
			}
			parts = append(parts, StrPart{IsExpr: true, Src: src, Start: exprStart})
			continue
		}
		r := lx.advance()
		lit.WriteRune(r)
	}
	if lit.Len() > 0 || len(parts) == 0 {
		parts = append(parts, StrPart{Lit: lit.String()})
	}
	t := lx.emit(STRING, lx.src[start.Offset-lx.baseOff:lx.off], start)
	t.Parts = parts
}

// skipNestedString advances over a string literal that appears inside an
// interpolation, including its own nested interpolations.
func (lx *lexer) skipNestedString() {
	lx.advance() // opening quote
	for lx.off < len(lx.src) && lx.src[lx.off] != '\n' {
		switch c := lx.src[lx.off]; {
		case c == '\\':
			lx.advance()
			if lx.off < len(lx.src) {
				lx.advance()
			}
		case c == '"':
			lx.advance()
			return
		case c == '$' && lx.peekc(1) == '{':
			lx.advance()
			lx.advance()
			depth := 0
			for lx.off < len(lx.src) && lx.src[lx.off] != '\n' {
				ch := lx.src[lx.off]
				if ch == '"' {
					lx.skipNestedString()
					continue
				}
				if ch == '{' {
					depth++
				} else if ch == '}' {
					if depth == 0 {
						lx.advance()
						break
					}
					depth--
				}
				lx.advance()
			}
		default:
			lx.advance()
		}
	}
}

// rawString lexes """...""": no escapes, no interpolation. A newline directly
// after the opening quotes is dropped, as are a final whitespace-only line
// (the indentation before the closing quotes) and common indentation.
func (lx *lexer) rawString(start diag.Pos) {
	for i := 0; i < 3; i++ {
		lx.advance()
	}
	b := lx.off
	for {
		if lx.off >= len(lx.src) {
			lx.errf(start, `unterminated raw string; expected """`)
			break
		}
		if strings.HasPrefix(lx.src[lx.off:], `"""`) {
			break
		}
		lx.advance()
	}
	body := lx.src[b:lx.off]
	if lx.off < len(lx.src) {
		for i := 0; i < 3; i++ {
			lx.advance()
		}
	}
	t := lx.emit(STRING, lx.src[start.Offset-lx.baseOff:lx.off], start)
	t.Parts = []StrPart{{Lit: dedent(body)}}
	t.Raw = true
}

func dedent(s string) string {
	s = strings.TrimPrefix(s, "\r")
	s = strings.TrimPrefix(s, "\n")
	lines := strings.Split(s, "\n")
	// Last line containing only whitespace (before closing quotes) sets no content.
	if len(lines) > 1 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	minIndent := -1
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, " \t"))
		if minIndent < 0 || n < minIndent {
			minIndent = n
		}
	}
	if minIndent > 0 {
		for i, l := range lines {
			if len(l) >= minIndent {
				lines[i] = l[minIndent:]
			} else {
				lines[i] = strings.TrimLeft(l, " \t")
			}
		}
	}
	return strings.Join(lines, "\n")
}

func (lx *lexer) punct(start diag.Pos) {
	two := ""
	if lx.off+2 <= len(lx.src) {
		two = lx.src[lx.off : lx.off+2]
	}
	if strings.HasPrefix(lx.src[lx.off:], "???") {
		lx.advance()
		lx.advance()
		lx.advance()
		lx.emit(HOLE, "???", start)
		return
	}
	twoKinds := map[string]Kind{"..": DOTDOT, "->": ARROW, "=>": FATARROW, "|>": PIPE, "==": EQ, "!=": NE, "<=": LE, ">=": GE}
	if k, ok := twoKinds[two]; ok {
		lx.advance()
		lx.advance()
		lx.emit(k, two, start)
		return
	}
	c := lx.src[lx.off]
	if two == "||" {
		lx.advance()
		lx.advance()
		lx.errf(start, "`||` is not a Veld operator").WithFix("use `or`", diag.Span{File: lx.file, Start: start, End: lx.pos()}, "or")
		return
	}
	one := map[byte]Kind{'(': LPAREN, ')': RPAREN, '[': LBRACK, ']': RBRACK, '{': LBRACE, '}': RBRACE, ',': COMMA,
		':': COLON, '.': DOT, '=': ASSIGN, '<': LT, '>': GT, '+': PLUS, '-': MINUS, '*': STAR, '/': SLASH,
		'%': PERCENT, '?': QUESTION, '|': BAR}
	if k, ok := one[c]; ok {
		lx.advance()
		switch k {
		case LPAREN, LBRACK, LBRACE:
			lx.depth++
		case RPAREN, RBRACK, RBRACE:
			if lx.depth > 0 {
				lx.depth--
			}
		}
		lx.emit(k, string(c), start)
		return
	}
	r := lx.advance()
	d := lx.errf(start, "unexpected character %q", r)
	switch {
	case two == "&&":
		lx.advance()
		d.Message = "`&&` is not a Veld operator"
		d.WithFix("use `and`", diag.Span{File: lx.file, Start: start, End: lx.pos()}, "and")
	case two == "||":
		lx.advance()
		d.Message = "`||` is not a Veld operator"
		d.WithFix("use `or`", diag.Span{File: lx.file, Start: start, End: lx.pos()}, "or")
	case r == '!':
		d.Message = "`!` is not a Veld operator"
		d.WithFix("use `not`", diag.Span{File: lx.file, Start: start, End: lx.pos()}, "not ")
	case r == ';':
		d.Message = "semicolons are not used in Veld; statements end at newlines"
		d.WithFix("remove the semicolon", diag.Span{File: lx.file, Start: start, End: lx.pos()}, "")
	case r == '\'':
		d.Note(`strings use double quotes: "text"`)
	case unicode.IsLetter(r):
		d.Note("identifiers must be ASCII: [a-zA-Z_][a-zA-Z0-9_]*")
	}
}
