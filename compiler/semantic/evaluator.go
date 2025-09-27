package semantic

import (
	"errors"
	"fmt"
	"strings"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/rungen"
	"github.com/brimdata/super/compiler/semantic/sem"
	"github.com/brimdata/super/sup"
)

type evaluator struct {
	translator *translator
	in         map[string]*sem.FuncDef
	errs       []errloc
	constThis  bool
	bad        bool
}

type errloc struct {
	loc ast.Node
	err error
}

func newEvaluator(t *translator, funcs map[string]*sem.FuncDef) *evaluator {
	return &evaluator{
		translator: t,
		in:         funcs,
	}
}

func (e *evaluator) mustEval(sctx *super.Context, expr sem.Expr) (super.Value, bool) {
	val, ok := e.maybeEval(sctx, expr)
	e.flushErrs()
	return val, ok
}

func (e *evaluator) maybeEval(sctx *super.Context, expr sem.Expr) (super.Value, bool) {
	if literal, ok := expr.(*sem.LiteralExpr); ok {
		val, err := sup.ParseValue(sctx, literal.Value)
		if err != nil {
			e.error(literal.Node, err)
			return val, false
		}
		return val, true
	}
	// re-enter the semantic analyzer with just this expr by resolving
	// all needed funcs then traversing the resulting sem tree and seeing
	// if it will eval as a compile-time constant.  If so, compile it the rest
	// of the way and invoke rungen to get the result and return it.
	// If an error is encountered, returns the error.  If the expression
	// isn't a compile-time const, then errors will accumulate.  Note that
	// no existing state in the translator is touched nor is the passed-in
	// sem tree modified at all; instead, the process here creates copies
	// of any needed sem tree and funcs.
	r := newResolver(e.translator)
	resolvedExpr, funcs := r.resolveExpr(expr)
	e.expr(resolvedExpr)
	if len(e.errs) > 0 || e.bad {
		return super.Value{}, false
	}
	for _, f := range funcs {
		e.constThis = true
		e.expr(f.Body)
		if len(e.errs) > 0 || e.bad {
			return super.Value{}, false
		}
	}
	main := newDagen(e.translator.reporter).assembleExpr(resolvedExpr, funcs)
	val, err := rungen.EvalAtCompileTime(sctx, main)
	if err != nil {
		e.error(expr, err)
		return val, false
	}
	return val, true
}

func (e *evaluator) seq(seq sem.Seq) bool {
	save := e.constThis
	for _, op := range seq {
		e.constThis = e.op(op)
	}
	constThis := e.constThis
	e.constThis = save
	return constThis
}

func (e *evaluator) op(op sem.Op) bool {
	switch op := op.(type) {
	//
	// Scanners first
	//
	case *sem.DefaultScan:
		return false
	case *sem.FileScan,
		*sem.HTTPScan,
		*sem.PoolScan,
		*sem.RobotScan,
		*sem.DBMetaScan,
		*sem.PoolMetaScan,
		*sem.CommitMetaScan,
		*sem.DeleteScan:
		e.error(op, errors.New("cannot read data in constant expression"))
		return false
	case *sem.NullScan:
		return true
	//
	// Ops in alphabetical oder
	//
	case *sem.AggregateOp:
		return e.assignments(op.Keys) && e.assignments(op.Aggs)
	case *sem.BadOp:
		e.bad = true
		return false
	case *sem.CutOp:
		return e.assignments(op.Args)
	case *sem.DebugOp:
		return e.expr(op.Expr) && e.constThis
	case *sem.DistinctOp:
		return e.expr(op.Expr) && e.constThis
	case *sem.DropOp:
		return e.exprs(op.Args) && e.constThis
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

func (e *evaluator) assignments(assignments []sem.Assignment) bool {
	isConst := true
	for _, a := range assignments {
		if !e.expr(a.LHS) { //XXX lval needs to be treated differently...
			isConst = false
		}
		if !e.expr(a.RHS) {
			isConst = false
		}
	}
	return isConst
}

func (e *evaluator) sortExprs(exprs []sem.SortExpr) bool {
	isConst := true
	for _, se := range exprs {
		if !e.expr(se.Expr) {
			isConst = false
		}
	}
	return isConst
}

func (e *evaluator) exprs(exprs []sem.Expr) bool {
	isConst := true
	for _, expr := range exprs {
		if !e.expr(expr) {
			isConst = false
		}
	}
	return isConst
}

func (e *evaluator) expr(expr sem.Expr) bool {
	switch expr := expr.(type) {
	case nil:
		return true
	case *sem.AggFunc:
		return e.expr(expr.Expr) && e.expr(expr.Where)
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

func quotedPath(path []string) string {
	if len(path) == 0 {
		return "this"
	}
	var elems []string
	for _, s := range path {
		elems = append(elems, sup.QuotedName(s))
	}
	return strings.Join(elems, ".")
}

func (e *evaluator) arrayElems(elems []sem.ArrayElem) bool {
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

func (e *evaluator) recordElems(elems []sem.RecordElem) bool {
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

func (e *evaluator) error(loc ast.Node, err error) {
	e.errs = append(e.errs, errloc{loc, err})
}

func (e *evaluator) flushErrs() {
	for _, info := range e.errs {
		e.translator.error(info.loc, info.err)
	}
}
