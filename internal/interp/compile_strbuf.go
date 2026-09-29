package interp

import (
	"unsafe"

	"github.com/ashraf82de/veld/internal/syntax"
)

// String building in place.
//
// `set s = s + a + b` on a local Str `var` is the idiom for building text, and
// concatenating into a fresh string each time is quadratic. The compiler marks
// such a variable as a string accumulator: its slot holds either a plain string
// or a *strBuf, and the update appends to the buffer. Reading the variable
// yields a string that aliases the buffer without copying. That is safe because
// bytes below a buffer's length are never modified afterwards: appends only
// write beyond every string handed out so far, and growth copies into a new
// array.

type strBuf struct{ b []byte }

func (s *strBuf) str() string {
	if len(s.b) == 0 {
		return ""
	}
	return unsafe.String(unsafe.SliceData(s.b), len(s.b))
}

// concatOperands returns the operands of `x + a + b` (left to right) when every
// addition is a Str addition and the leftmost operand is the variable name.
func (c *compiler) concatOperands(e syntax.Expr, name string) ([]syntax.Expr, bool) {
	var rest []syntax.Expr
	for {
		b, ok := e.(*syntax.BinaryExpr)
		if !ok || b.Op != syntax.PLUS || !c.is(b, "Str") {
			break
		}
		rest = append([]syntax.Expr{b.R}, rest...)
		e = b.L
	}
	id, ok := e.(*syntax.Ident)
	if !ok || id.Name != name || len(rest) == 0 {
		return nil, false
	}
	return rest, true
}

// inplaceStr compiles `set s = s + a + ...` for a local Str var. Like inplace,
// the first pass only records the variable; later passes emit the append.
func (c *compiler) inplaceStr(s *syntax.SetStmt) (code, bool) {
	if !c.is(s.Value, "Str") {
		return nil, false
	}
	operands, ok := c.concatOperands(s.Value, s.Name)
	if !ok {
		return nil, false
	}
	v := c.lookup(s.Name)
	if v == nil || v.decl == nil || !v.mutable || v.boxed {
		return nil, false
	}
	if !c.strOwned[v.decl] {
		c.strOwned[v.decl] = true
		c.grew = true
		return nil, false
	}
	for _, x := range operands {
		if c.mayExit(x) {
			return nil, false
		}
	}
	parts := make([]func(f *frame) string, len(operands))
	for i, x := range operands {
		parts[i] = c.strExpr(x)
	}
	slot := v.slot
	return func(f *frame) Value {
		// Evaluate every operand first: they may read the variable, and they
		// must see the text as it was before this update.
		var vals [4]string
		var many []string
		var operandsText []string
		if len(parts) <= len(vals) {
			operandsText = vals[:len(parts)]
		} else {
			many = make([]string, len(parts))
			operandsText = many
		}
		total := 0
		for i, p := range parts {
			operandsText[i] = p(f)
			total += len(operandsText[i])
		}
		buf, isBuf := f.slots[slot].(*strBuf)
		if !isBuf {
			cur, _ := f.slots[slot].(string)
			buf = &strBuf{b: make([]byte, 0, max(64, 2*(len(cur)+total)))}
			buf.b = append(buf.b, cur...)
			f.slots[slot] = buf
		}
		for _, t := range operandsText {
			buf.b = append(buf.b, t...)
		}
		return Unit
	}, true
}
