package semantic

import (
	"fmt"

	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/semantic/sem"
)

// Column of a select statement.  We bookkeep here whether
// a column is a scalar expression or an aggregation by looking up the function
// name and seeing if it's an aggregator or not.  We also infer the column
// names so we can do SQL error checking relating the selections to the grouping keys,
// and statically compute the resulting schema from the selection.
// When the column is an agg function expression,
// the expression is composed of agg functions and
// fixed references relative to the agg (like grouping keys)
// as well as alias from selected columns to the left of the
// agg expression.  e.g., select max(x) m, (sum(a) + sum(b)) / m as q
// would be two aggs where sum(a) and sum(b) are
// stored inside the aggs slice and we subtitute temp variables for
// the agg functions in the expr field.
type column struct {
	name  string
	loc   ast.Node
	expr  sem.Expr
	isAgg bool
}

type projection []column

func (p projection) hasStar() bool {
	for _, col := range p {
		if col.isStar() {
			return true
		}
	}
	return false
}

func (p projection) aggCols() []column {
	aggs := make([]column, 0, len(p))
	for _, col := range p {
		if col.isAgg {
			aggs = append(aggs, col)
		}
	}
	return aggs
}

func newColumn(name string, loc ast.Expr, e sem.Expr, funcs *aggfuncs) *column {
	c := &column{name: name, loc: loc}
	cnt := len(*funcs)
	c.expr = funcs.subst(e)
	c.isAgg = cnt != len(*funcs)
	return c
}

func (c *column) isStar() bool {
	return c.expr == nil
}

// namedAgg gives us a place to bind temp name to each agg function.
type namedAgg struct {
	name string
	agg  *sem.AggFunc
}

type aggfuncs []namedAgg

func (a aggfuncs) tmp() string {
	return fmt.Sprintf("t%d", len(a))
}

func (a *aggfuncs) subst(e sem.Expr) sem.Expr {
	return exprWalk(e, func(e sem.Expr) (sem.Expr, bool) {
		switch e := e.(type) {
		case *sem.AggFunc:
			// swap in a temp column for each agg function found, which
			// will then be referred to by the containing expression.
			// The agg function is computed into the tmp value with
			// the generated aggregate operator.
			tmp := a.tmp()
			*a = append(*a, namedAgg{name: tmp, agg: e})
			return sem.NewThis(e, []string{"in", tmp}), true
		default:
			return e, false
		}
	})
}

func keySubst(e sem.Expr, exprs []exprloc) (sem.Expr, bool) {
	ok := true
	e = exprWalk(e, func(e sem.Expr) (sem.Expr, bool) {
		if i := exprMatch(e, exprs); i >= 0 {
			return sem.NewThis(e, []string{"in", fmt.Sprintf("k%d", i)}), true
		}
		switch e := e.(type) {
		case *sem.ThisExpr:
			ok = false
		case *sem.AggFunc:
			// This should't happen.
			panic(e)
		}
		return e, false
	})
	return e, ok
}

type exprVisitor func(e sem.Expr) (sem.Expr, bool)

func exprWalk(e sem.Expr, visit exprVisitor) sem.Expr {
	var stop bool
	if e, stop = visit(e); stop {
		return e
	}
	switch e := e.(type) {
	case nil:
	case *sem.AggFunc:
		e.Expr = exprWalk(e, visit)
		e.Filter = exprWalk(e, visit)
	case *sem.ArrayExpr:
		e.Elems = exprWalkArrayElems(e.Elems, visit)
	case *sem.BadExpr:
	case *sem.BinaryExpr:
		e.LHS = exprWalk(e.LHS, visit)
		e.RHS = exprWalk(e.RHS, visit)
	case *sem.CallExpr:
		var out []sem.Expr
		for _, arg := range e.Args {
			out = append(out, exprWalk(arg, visit))
		}
		e.Args = out
	case *sem.CondExpr:
		e.Cond = exprWalk(e.Cond, visit)
		e.Then = exprWalk(e.Then, visit)
		e.Else = exprWalk(e.Else, visit)
	case *sem.DotExpr:
		e.LHS = exprWalk(e.LHS, visit)
	case *sem.IndexExpr:
		e.Expr = exprWalk(e.Expr, visit)
		e.Index = exprWalk(e.Index, visit)
	case *sem.IsNullExpr:
		e.Expr = exprWalk(e.Expr, visit)
	case *sem.LiteralExpr:
	case *sem.MapExpr:
		for _, ent := range e.Entries {
			ent.Key = exprWalk(ent.Key, visit)
			ent.Value = exprWalk(ent.Value, visit)
		}
	case *sem.RecordExpr:
		var out []sem.RecordElem
		for _, elem := range e.Elems {
			switch elem := elem.(type) {
			case *sem.FieldElem:
				e := exprWalk(elem.Value, visit)
				out = append(out, &sem.FieldElem{Node: elem, Name: elem.Name, Value: e})
			case *sem.SpreadElem:
				e := exprWalk(elem.Expr, visit)
				out = append(out, &sem.SpreadElem{Node: elem, Expr: e})
			default:
				panic(elem)
			}
		}
		e.Elems = out
	case *sem.RegexpMatchExpr:
		e.Expr = exprWalk(e.Expr, visit)
	case *sem.RegexpSearchExpr:
		e.Expr = exprWalk(e.Expr, visit)
	case *sem.SearchTermExpr:
		e.Expr = exprWalk(e.Expr, visit)
	case *sem.SetExpr:
		e.Elems = exprWalkArrayElems(e.Elems, visit)
	case *sem.SliceExpr:
		e.Expr = exprWalk(e.Expr, visit)
		e.From = exprWalk(e.From, visit)
		e.To = exprWalk(e.To, visit)
	case *sem.SubqueryExpr: // XXX This might need to be traversed?
	case *sem.ThisExpr:
	case *sem.UnaryExpr:
		e.Operand = exprWalk(e.Operand, visit)
	default:
		panic(e)
	}
	return e
}

func exprWalkArrayElems(elems []sem.ArrayElem, visit exprVisitor) []sem.ArrayElem {
	var out []sem.ArrayElem
	for _, elem := range elems {
		switch elem := elem.(type) {
		case *sem.SpreadElem:
			e := exprWalk(elem.Expr, visit)
			out = append(out, &sem.SpreadElem{Node: elem.Node, Expr: e})
		case *sem.ExprElem:
			e := exprWalk(elem.Expr, visit)
			out = append(out, &sem.ExprElem{Node: elem.Node, Expr: e})
		default:
			panic(elem)
		}
	}
	return out
}
