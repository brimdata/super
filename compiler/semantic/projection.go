package semantic

import (
	"errors"
	"fmt"

	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/dag"
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
	loc   ast.Expr
	expr  dag.Expr
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

func newColumn(name string, loc ast.Expr, e dag.Expr, funcs *aggfuncs) (*column, error) {
	c := &column{name: name, loc: loc}
	cnt := len(*funcs)
	var err error
	c.expr, err = funcs.subst(e)
	if err != nil {
		return nil, err
	}
	c.isAgg = cnt != len(*funcs)
	return c, nil
}

func (c *column) isStar() bool {
	return c.expr == nil
}

// namedAgg gives us a place to bind temp name to each agg function.
type namedAgg struct {
	name string
	agg  *dag.Agg
}

type aggfuncs []namedAgg

func (a aggfuncs) tmp() string {
	return fmt.Sprintf("t%d", len(a))
}

func (a *aggfuncs) subst(e dag.Expr) (dag.Expr, error) {
	var err error
	switch e := e.(type) {
	case nil:
		return e, nil
	case *dag.Agg:
		// swap in a temp column for each agg function found, which
		// will then be referred to by the containing expression.
		// The agg function is computed into the tmp value with
		// the generated aggregate operator.
		tmp := a.tmp()
		*a = append(*a, namedAgg{name: tmp, agg: e})
		return &dag.This{Kind: "This", Path: []string{"in", tmp}}, nil
	case *dag.ArrayExpr:
		for _, elem := range e.Elems {
			switch elem := elem.(type) {
			case *dag.Spread:
				elem.Expr, err = a.subst(elem.Expr)
				if err != nil {
					return nil, err
				}
			case *dag.VectorValue:
				elem.Expr, err = a.subst(elem.Expr)
				if err != nil {
					return nil, err
				}
			default:
				panic(elem)
			}
		}
	case *dag.BinaryExpr:
		e.LHS, err = a.subst(e.LHS)
		if err != nil {
			return nil, err
		}
		e.RHS, err = a.subst(e.RHS)
		if err != nil {
			return nil, err
		}
	case *dag.Call:
		for k, arg := range e.Args {
			e.Args[k], err = a.subst(arg)
			if err != nil {
				return nil, err
			}
		}
	case *dag.Conditional:
		e.Cond, err = a.subst(e.Cond)
		if err != nil {
			return nil, err
		}
		e.Then, err = a.subst(e.Then)
		if err != nil {
			return nil, err
		}
		e.Else, err = a.subst(e.Else)
		if err != nil {
			return nil, err
		}
	case *dag.Dot:
		e.LHS, err = a.subst(e.LHS)
		if err != nil {
			return nil, err
		}
	case *dag.IndexExpr:
		e.Expr, err = a.subst(e.Expr)
		if err != nil {
			return nil, err
		}
		e.Index, err = a.subst(e.Index)
		if err != nil {
			return nil, err
		}
	case *dag.IsNullExpr:
		e.Expr, err = a.subst(e.Expr)
		if err != nil {
			return nil, err
		}
	case *dag.Literal:
	case *dag.MapCall:
		e.Expr, err = a.subst(e.Expr)
		if err != nil {
			return nil, err
		}
	case *dag.MapExpr:
		for _, ent := range e.Entries {
			ent.Key, err = a.subst(ent.Key)
			if err != nil {
				return nil, err
			}
			ent.Value, err = a.subst(ent.Value)
			if err != nil {
				return nil, err
			}
		}
	case *dag.RecordExpr:
		for _, elem := range e.Elems {
			switch elem := elem.(type) {
			case *dag.Field:
				elem.Value, err = a.subst(elem.Value)
				if err != nil {
					return nil, err
				}
			case *dag.Spread:
				elem.Expr, err = a.subst(elem.Expr)
				if err != nil {
					return nil, err
				}
			default:
				panic(elem)
			}
		}
	case *dag.RegexpMatch:
		e.Expr, err = a.subst(e.Expr)
		if err != nil {
			return nil, err
		}
	case *dag.RegexpSearch:
		e.Expr, err = a.subst(e.Expr)
		if err != nil {
			return nil, err
		}
	case *dag.Search:
		e.Expr, err = a.subst(e.Expr)
		if err != nil {
			return nil, err
		}
	case *dag.SetExpr:
		for _, elem := range e.Elems {
			switch elem := elem.(type) {
			case *dag.Spread:
				elem.Expr, err = a.subst(elem.Expr)
				if err != nil {
					return nil, err
				}
			case *dag.VectorValue:
				elem.Expr, err = a.subst(elem.Expr)
				if err != nil {
					return nil, err
				}
			default:
				panic(elem)
			}
		}
	case *dag.SliceExpr:
		e.Expr, err = a.subst(e.Expr)
		if err != nil {
			return nil, err
		}
		e.From, err = a.subst(e.From)
		if err != nil {
			return nil, err
		}
		e.To, err = a.subst(e.To)
		if err != nil {
			return nil, err
		}
	case *dag.This:
	case *dag.UnaryExpr:
		e.Operand, err = a.subst(e.Operand)
		if err != nil {
			return nil, err
		}
	case *dag.UnnestExpr:
		return nil, errors.New("unnest expression not allowed with aggregate function")
	case *dag.Var:
		return nil, errors.New("var not allowed with aggregate function")
	}
	return e, nil
}
