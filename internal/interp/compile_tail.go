package interp

import "github.com/ashraf82de/veld/internal/syntax"

// Tail calls. A call to a user function in tail position (the last thing a
// function does, possibly inside if/match branches or after `return`) does not
// grow the stack: the compiled call parks the callee's frame in the caller's
// frame and returns, and Thread.invoke runs the callee in its own loop. Mutual
// recursion and recursive loops therefore run in constant stack. Frames of
// tail-called functions do not appear in stack traces.

// markTailBlock marks the calls in tail position of a function body.
func (c *compiler) markTailBlock(b *syntax.Block) {
	if b == nil || len(b.Stmts) == 0 {
		return
	}
	if es, ok := b.Stmts[len(b.Stmts)-1].(*syntax.ExprStmt); ok {
		c.markTailExpr(es.X)
	}
}

func (c *compiler) markTailExpr(e syntax.Expr) {
	switch e := e.(type) {
	case *syntax.CallExpr, *syntax.PipeExpr:
		c.tailNodes[e] = true
	case *syntax.ParenExpr:
		c.markTailExpr(e.X)
	case *syntax.IfExpr:
		for _, br := range e.Branches {
			c.markTailBlock(br.Body)
		}
		c.markTailBlock(e.Else)
	case *syntax.MatchExpr:
		for _, a := range e.Arms {
			c.markTailBlock(a.Body)
		}
	case *syntax.ReturnExpr:
		if e.X != nil {
			c.markTailExpr(e.X)
		}
	}
}

// tailCallee returns the user function a call expression targets, if it is
// eligible to be compiled as a tail call.
func (c *compiler) tailCallee(e syntax.Expr) *fnCode {
	for {
		p, ok := e.(*syntax.ParenExpr)
		if !ok {
			break
		}
		e = p.X
	}
	var callFn syntax.Expr
	switch e := e.(type) {
	case *syntax.CallExpr:
		callFn = e.Fn
	case *syntax.PipeExpr:
		callFn = e.R.Fn
	default:
		return nil
	}
	if c.ctorOfCallee(callFn) != nil {
		return nil
	}
	fi := c.staticFunc(callFn)
	if fi == nil || fi.Extern {
		return nil
	}
	return c.in.fns[fi]
}
