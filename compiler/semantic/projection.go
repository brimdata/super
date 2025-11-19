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

func newColumn(name string, loc ast.Expr, e sem.Expr, funcs *aggfuncs) (*column, error) {
	c := &column{name: name, loc: loc}
	cnt := len(*funcs)
	c.expr = funcs.subst(e)
	c.isAgg = cnt != len(*funcs)
	return c, nil
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
	switch e := e.(type) {
	case nil:
		return e
	case *sem.AggFunc:
		// swap in a temp column for each agg function found, which
		// will then be referred to by the containing expression.
		// The agg function is computed into the tmp value with
		// the generated aggregate operator.
		tmp := a.tmp()
		*a = append(*a, namedAgg{name: tmp, agg: e})
		return sem.NewThis(e, []string{"in", tmp})
	case *sem.ArrayExpr:
		e.Elems = a.substArrayElems(e.Elems)
	case *sem.BinaryExpr:
		e.LHS = a.subst(e.LHS)
		e.RHS = a.subst(e.RHS)
	case *sem.CallExpr:
		for k, arg := range e.Args {
			e.Args[k] = a.subst(arg)
		}
	case *sem.CondExpr:
		e.Cond = a.subst(e.Cond)
		e.Then = a.subst(e.Then)
		e.Else = a.subst(e.Else)
	case *sem.DotExpr:
		e.LHS = a.subst(e.LHS)
	case *sem.IndexExpr:
		e.Expr = a.subst(e.Expr)
		e.Index = a.subst(e.Index)
	case *sem.IsNullExpr:
		e.Expr = a.subst(e.Expr)
	case *sem.LiteralExpr:
	case *sem.MapExpr:
		for _, ent := range e.Entries {
			ent.Key = a.subst(ent.Key)
			ent.Value = a.subst(ent.Value)
		}
	case *sem.RecordExpr:
		var elems []sem.RecordElem
		for _, elem := range e.Elems {
			switch elem := elem.(type) {
			case *sem.FieldElem:
				sub := a.subst(elem.Value)
				elems = append(elems, &sem.FieldElem{Node: elem, Name: elem.Name, Value: sub})
			case *sem.SpreadElem:
				elems = append(elems, &sem.SpreadElem{Node: elem, Expr: a.subst(elem.Expr)})
			default:
				panic(elem)
			}
		}
		e.Elems = elems
	case *sem.RegexpMatchExpr:
		e.Expr = a.subst(e.Expr)
	case *sem.RegexpSearchExpr:
		e.Expr = a.subst(e.Expr)
	case *sem.SearchTermExpr:
		e.Expr = a.subst(e.Expr)
	case *sem.SetExpr:
		e.Elems = a.substArrayElems(e.Elems)
	case *sem.SliceExpr:
		e.Expr = a.subst(e.Expr)
		e.From = a.subst(e.From)
		e.To = a.subst(e.To)
	case *sem.ThisExpr:
	case *sem.UnaryExpr:
		e.Operand = a.subst(e.Operand)
	}
	return e
}

func (a *aggfuncs) substArrayElems(elems []sem.ArrayElem) []sem.ArrayElem {
	var out []sem.ArrayElem
	for _, e := range elems {
		switch e := e.(type) {
		case *sem.SpreadElem:
			out = append(out, &sem.SpreadElem{Node: e, Expr: a.subst(e.Expr)})
		case *sem.ExprElem:
			out = append(out, &sem.ExprElem{Node: e, Expr: a.subst(e.Expr)})
		default:
			panic(e)
		}
	}
	return out
}
