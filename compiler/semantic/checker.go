package semantic

import (
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/semantic/sem"
)

type checker struct {
	reporter
	sctx  *super.Context //XXX?
	funcs map[string]*sem.FuncDef
	errs  []errloc
	bad   bool
}

func newChecker(r reporter, sctx *super.Context, funcs map[string]*sem.FuncDef) *checker {
	return &checker{
		reporter: r,
		sctx:     sctx,
		funcs:    funcs,
	}
}

/* model is to do the type checking bottom up during the translation pass
so we don't need this...
func (c *checker) seq(typ super.Type, seq sem.Seq) super.Type {
	for _, op := range seq {
		typ = c.op(typ, op)
	}
	return typ
}
*/

func (c *checker) op(typ super.Type, op sem.Op) super.Type {
	switch op := op.(type) {
	//
	// Scanners first
	//
	case *sem.DefaultScan:
		return super.TypeNull //XXX should get type from readers interface
	case *sem.FileScan:
		// XXX should have been set by translator so that SQL schemas could b e
		// managed
		return op.GetType()
	case *sem.HTTPScan,
		*sem.PoolScan,
		*sem.RobotScan,
		*sem.DBMetaScan,
		*sem.NullScan,
		*sem.PoolMetaScan,
		*sem.CommitMetaScan,
		*sem.DeleteScan:
		op.SetType(super.TypeNull)
		return super.TypeNull
	//
	// Ops in alphabetical oder
	//
	case *sem.AggregateOp:
		return e.assignments(op.Keys) && e.assignments(op.Aggs)
	case *sem.BadOp:
		c.bad = true
		return super.TypeNull
	case *sem.CutOp:
		return c.cutOp(op)
	case *sem.DebugOp:
		op.SetType(typ)
		//XXX do analysis on debug expr
		return typ
	case *sem.DistinctOp:
		typ = c.expr(typ, op.Expr)
		op.SetType(typ)
		return typ
	case *sem.DropOp:
		//XXX need to get cut fields and synthesize dropped type
		// XXX should have a types library to do this?  share code with drop?
		typ = super.TypeNull
		op.SetType(typ)
		return typ
	case *sem.ExplodeOp:
		return e.exprs(op.Args) && e.constThis
	case *sem.FilterOp:
		return e.expr(op.Expr) && e.constThis
	case *sem.ForkOp:
		isConst := true
		for _, seq := range op.Paths {
			if !e.seq(seq) {
				isConst = false
			}
		}
		return isConst
	case *sem.FuseOp:
		return e.constThis
	case *sem.HeadOp:
		return e.constThis
	case *sem.JoinOp:
		// This join depends on the parents but this is handled in the fork above.
		// If any path of parents are non-const, then constThis will be false on
		// entering here.
		return e.expr(op.Cond) && e.constThis
	case *sem.LoadOp:
		return true
	case *sem.MergeOp:
		// Like join, if any of the parent legs is non-const, the constThis if false here
		return e.sortExprs(op.Exprs) && e.constThis
	case *sem.OutputOp:
		return true
	case *sem.PutOp:
		return e.assignments(op.Args) && e.constThis
	case *sem.RenameOp:
		return e.assignments(op.Args) && e.constThis
	case *sem.SkipOp:
		return e.constThis
	case *sem.SortOp:
		return e.sortExprs(op.Exprs) && e.constThis
	case *sem.SwitchOp:
		e.constThis = e.expr(op.Expr)
		isConst := true
		for _, c := range op.Cases {
			if !e.expr(c.Expr) {
				isConst = false
			}
			if !e.seq(c.Path) {
				isConst = false
			}
		}
		return isConst
	case *sem.TailOp:
		return e.constThis
	case *sem.TopOp:
		return e.sortExprs(op.Exprs) && e.constThis
	case *sem.UniqOp:
		return e.constThis
	case *sem.UnnestOp:
		e.constThis = e.expr(op.Expr)
		return e.seq(op.Body)
	case *sem.ValuesOp:
		return e.exprs(op.Exprs)
	default:
		panic(op)
	}
}

func (c *checker) cutOp(in super.Type, cut *sem.CutOp) super.Type {
	types := c.assignments(in, cut.Args)
	var fields []super.Field 
	for _, t : =range types {
		fields = append(fields &super.Field{

		}
	}
	op.SetType(typ)
	return typ
}

func (c *checker) assignments(typ super.Type, assignments []sem.Assignment) []super.Fields {
	for _, a := range assignments {
		
	}
	return isConst
}

func (c *checker) sortExprs(typ super.Type, exprs []sem.SortExpr) {
	isConst := true
	for _, se := range exprs {
		if !e.expr(se.Expr) {
			isConst = false
		}
	}
	return isConst
}

func (c *checker) exprs(typ super.Type, exprs []sem.Expr) []super.Type {
	var types []super.Type
	for _, e := range exprs {
		types = append(types, c.expr(typ, e))
	}
	return types
}

func (c *checker) expr(typ super.Type, e sem.Expr) super.Type {
	switch e := e.(type) {
	case nil:
		return super.TypeNull
	case *sem.AggFunc:
		return super.TypeNull
		//return c.expr(expr.Expr) && c.expr(expr.Where)
	case *sem.ArrayExpr:
		return e.arrayElems(expr.Elems)
	case *sem.BadExpr:
		e.bad = true
		return false
	case *sem.BinaryExpr:
		return e.expr(expr.LHS) && e.expr(expr.RHS)
	case *sem.CallExpr:
		// XXX should calls with side-effects not be const?
		// once you're in the call, you're good.  but the body must not
		// do a subquery with ext input.  so we need to scan the funcs.
		// this means e.funcs should be here to check.
		return e.exprs(expr.Args)
	case *sem.CondExpr:
		return e.expr(expr.Cond) && e.expr(expr.Then) && e.expr(expr.Else)
	case *sem.DotExpr:
		return e.expr(expr.LHS)
	case *sem.IndexExpr:
		return e.expr(expr.Expr) && e.expr(expr.Index)
	case *sem.IsNullExpr:
		return e.expr(expr.Expr)
	case *sem.LiteralExpr:
		return true
	case *sem.MapCallExpr:
		return e.expr(expr.Expr) && e.expr(expr.Lambda)
	case *sem.MapExpr:
		isConst := true
		for _, entry := range expr.Entries {
			if !e.expr(entry.Key) || !e.expr(entry.Value) {
				isConst = false
			}
		}
		return isConst
	case *sem.RecordExpr:
		return e.recordElems(expr.Elems)
	case *sem.RegexpMatchExpr:
		return e.expr(expr.Expr)
	case *sem.RegexpSearchExpr:
		return e.expr(expr.Expr)
	case *sem.SearchTermExpr:
		return e.expr(expr.Expr)
	case *sem.SetExpr:
		return e.arrayElems(expr.Elems)
	case *sem.SliceExpr:
		return e.expr(expr.Expr) && e.expr(expr.From) && e.expr(expr.To)
	case *sem.SubqueryExpr:
		//XXX fix this
		return e.seq(expr.Body)
	case *sem.ThisExpr:
		if !e.constThis {
			e.error(expr, fmt.Errorf("cannot reference '%s' in constant expression", quotedPath(expr.Path)))
		}
		return e.constThis
	case *sem.UnaryExpr:
		return e.expr(expr.Operand)
	default:
		panic(e)
	}
}

func (c *checker) arrayElems(elems []sem.ArrayElem) bool {
	isConst := true
	for _, elem := range elems {
		switch elem := elem.(type) {
		case *sem.SpreadElem:
			if !e.expr(elem.Expr) {
				isConst = false
			}
		case *sem.ExprElem:
			if !e.expr(elem.Expr) {
				isConst = false
			}
		default:
			panic(elem)
		}
	}
	return isConst
}

func (c *checker) recordElems(elems []sem.RecordElem) bool {
	isConst := true
	for _, elem := range elems {
		switch elem := elem.(type) {
		case *sem.SpreadElem:
			if !e.expr(elem.Expr) {
				isConst = false
			}
		case *sem.FieldElem:
			if !e.expr(elem.Value) {
				isConst = false
			}
		default:
			panic(elem)
		}
	}
	return isConst
}

// XXX share this with evaluator etc
func (c *checker) error(loc ast.Node, err error) {
	c.errs = append(c.errs, errloc{loc, err})
}

func (c *checker) flushErrs() {
	for _, info := range e.errs {
		c.reporter.error(info.loc, info.err)
	}
}
