package semantic

import (
	"fmt"

	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/pkg/field"
)

// For aggs, we have the following structure:
//
// {in:input_rel}
// {in:<input>,out:{c0:<expr>}}
// {in:<input>,out:{c0:<expr>,c1:<expr>}}
// where(in,out)
// summarize f1_1:=fn(<expr>),k1:=<expr>,k2:<expr>...,f1_k:=fn(<e>),h group keys, h agg funcs
// having = filter(haggs, group-keys)
// anon{projName1:<key or func>}, etc... in/out gone

type schema interface {
	Name() string
}

type schemaStatic struct {
	name    string
	columns []string
}

type schemaAnon struct {
	columns []string
}

type schemaDynamic struct {
	name string
}

type schemaSelect struct {
	in  schema
	out schema
}

type schemaJoin struct {
	left  schema
	right schema
}

func (s *schemaStatic) Name() string  { return s.name }
func (s *schemaDynamic) Name() string { return s.name }
func (s *schemaAnon) Name() string    { return "" }
func (s *schemaSelect) Name() string  { return "" }
func (s *schemaJoin) Name() string    { return "" }

func badSchema() schema {
	return &schemaDynamic{}
}

// Column of a select statement.  We bookkeep here whether
// a column is a scalar expression or an aggregation by looking up the function
// name and seeing if it's an aggregator or not.  We also infer the column
// names so we can do SQL error checking relating the selections to the group-by keys,
// and statically compute the resulting schema from the selection.
// When the column is an agg function expression,
// the expression is composed of agg functions and
// fixed references relative to the agg (like group-by keys)
// as well as alias from selected columns to the left of the
// agg expression.  e.g., select max(x) m, (sum(a) + sum(b)) / m as q
// would be two aggExprs where sum(a) and sum(b) are
// stored inside the second aggExpr.  They are given temp variable
// names so the expression be computed on exit from the summarize pipe
// operator, e.g.,
//
//	summarize t1:=max(x),t2:=sum(a),t3:=sum(b)
//	yield {m:t1}
//	yield {...this,q:(t1+t2)/m}
type column struct {
	name string
	expr dag.Expr
	aggs []namedAgg
}

type namedAgg struct {
	name string
	agg  *dag.Agg
}

type projection []column

func (p projection) hasStar() bool {
	for _, col := range p {
		if col.expr == nil {
			return true
		}
	}
	return false
}

func (p projection) aggExprs() []column {
	aggs := make([]column, 0, len(p))
	for _, col := range p {
		if len(col.aggs) != 0 {
			aggs = append(aggs, col)
		}
	}
	return aggs
}

// Return the scalar paths that are in the selection.
func (p projection) paths() field.List {
	var fields field.List
	for _, col := range p.scalarExprs() {
		if this, ok := col.expr.(*dag.This); ok {
			fields = append(fields, this.Path)
		}
	}
	return fields
}

func (p projection) scalarExprs() []column {
	scalars := make([]column, 0, len(p))
	for _, col := range p {
		if len(col.aggs) == 0 {
			scalars = append(scalars, col)
		}
	}
	return scalars
}

//XXX
// For each aggfunc, we'll gen fk:=f()
// For each aggexpr as v, we'll generate v:=expr(fk,...) from the aggs
// For the having expr, we'll filter on predicate after table gen'd,
// For each having aggfun, we'll gen hk:=f()

func newColumn(e dag.Expr, tm *tmpMaker) *column {
	c := &column{}
	c.expr = c.build(tm, e)
	return c
}

//XXX need to detect mixed aggfunc calls with scalars that aren't
// in the input to an aggfunc... two traversals?  but we should let
// path refs to aggfunc results or group-by keys in...
// so traversal should detect scalar terms that aren't in group-by
// (or if group-by clause not present or we have group-by all we can
// infer they are group-by keys)

//XXX come up for tests for all these cases

func (a *column) build(tm *tmpMaker, e dag.Expr) dag.Expr {
	switch e := e.(type) {
	case nil:
		return e
	case *dag.Agg:
		// swap in a temp column for each agg function found, which
		// will then be referred to by the containing expression.
		// The agg function is computed into the tmp value with
		// the generated summarize operator.
		tmp := tm.get()
		a.aggs = append(a.aggs, namedAgg{name: tmp, agg: e})
		return pathOf(tmp)
	case *dag.ArrayExpr:
		for _, elem := range e.Elems {
			switch elem := elem.(type) {
			case *dag.Spread:
				elem.Expr = a.build(tm, elem.Expr)
			case *dag.VectorValue:
				elem.Expr = a.build(tm, elem.Expr)
			default:
				panic(elem)
			}
		}
	case *dag.BinaryExpr:
		e.LHS = a.build(tm, e.LHS)
		e.RHS = a.build(tm, e.RHS)
	case *dag.Call:
		for k, arg := range e.Args {
			e.Args[k] = a.build(tm, arg)
		}
	case *dag.Conditional:
		e.Cond = a.build(tm, e.Cond)
		e.Then = a.build(tm, e.Then)
		e.Else = a.build(tm, e.Else)
	case *dag.Dot:
		e.LHS = a.build(tm, e.LHS)
	case *dag.Func:
		// XXX
	case *dag.IndexExpr:
		e.Expr = a.build(tm, e.Expr)
		e.Index = a.build(tm, e.Index)
	case *dag.IsNullExpr:
		e.Expr = a.build(tm, e.Expr)
	case *dag.Literal:
	case *dag.MapCall:
		e.Expr = a.build(tm, e.Expr)
	case *dag.MapExpr:
		for _, ent := range e.Entries {
			ent.Key = a.build(tm, ent.Key)
			ent.Value = a.build(tm, ent.Value)
		}
	case *dag.OverExpr:
		panic("TBD ERROR") //XXX
	case *dag.RecordExpr:
		for _, elem := range e.Elems {
			switch elem := elem.(type) {
			case *dag.Field:
				elem.Value = a.build(tm, elem.Value)
			case *dag.Spread:
				elem.Expr = a.build(tm, elem.Expr)
			default:
				panic(elem)
			}
		}
		return d
	case *dag.RegexpMatch:
		e.Expr = a.build(tm, e.Expr)
	case *dag.RegexpSearch:
		e.Expr = a.build(tm, e.Expr)
	case *dag.Search:
		e.Expr = a.build(tm, e.Expr)
	case *dag.SetExpr:
		for _, elem := range e.Elems {
			switch elem := elem.(type) {
			case *dag.Spread:
				elem.Expr = a.build(tm, elem.Expr)
			case *dag.VectorValue:
				elem.Expr = a.build(tm, elem.Expr)
			default:
				panic(elem)
			}
		}
	case *dag.SliceExpr:
		e.Expr = a.build(tm, e.Expr)
		e.From = a.build(tm, e.From)
		e.To = a.build(tm, e.To)
	case *dag.This:
	case *dag.UnaryExpr:
		e.Operand = a.build(tm, e.Operand)
	case *dag.Var:
	}
	return e
}

func (c *column) isStar() bool {
	return c.expr == nil
}

type tmpMaker int

func (t *tmpMaker) get() string {
	k := *t
	*t++
	return fmt.Sprintf("t%d", k)
}
