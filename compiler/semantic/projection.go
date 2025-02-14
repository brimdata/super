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
// names so we can do SQL error checking relating the selections to the group-by keys.
type column interface {
	Name() string
}

type scalarExpr struct {
	name string
	expr dag.Expr
}

func (s *scalarExpr) Name() string { return s.name }

type aggExpr struct {
	name   string
	prefix string
	aggs   []*dag.Agg
	expr   dag.Expr
}

func (a *aggExpr) Name() string { return a.name }

func (a *aggExpr) TempName(col, off int) string { return fmt.Sprintf("%s%d_%d", a.prefix, col, off) }

type starExpr struct{}

func (*starExpr) Name() string { return "" }

type projection []column

func (p projection) hasStar() bool {
	for _, col := range p {
		if _, ok := col.(*starExpr); ok {
			return true
		}
	}
	return false
}

func (p projection) aggExprs() []*aggExpr {
	aggs := make([]*aggExpr, 0, len(p))
	for _, col := range p {
		if a, ok := col.(*aggExpr); ok {
			aggs = append(aggs, a)
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

func (p projection) scalarExprs() []*scalarExpr {
	scalars := make([]*scalarExpr, 0, len(p))
	for _, col := range p {
		if s, ok := col.(*scalarExpr); ok {
			scalars = append(scalars, s)
		}
	}
	return scalars
}

// For each aggfunc, we'll gen fk:=f()
// For each aggexpr as v, we'll generate v:=expr(fk,...) from the aggs
// For the having expr, we'll filter on predicate after table gen'd,
// For each having aggfun, we'll gen hk:=f()

func (p projection) yieldScalars(seq dag.Seq, sch *schemaSelect) dag.Seq {
	if len(p) == 0 {
		return nil
	}
	for k, col := range p {
		var elems []dag.RecordElem
		if k != 0 {
			elems = append(elems, &dag.Spread{
				Kind: "Spread",
				Expr: &dag.This{Kind: "This", Path: []string{"out"}},
			})
		}
		if col.isStar() {
			for _, path := range unravel(sch, nil) {
				elems = append(elems, &dag.Spread{
					Kind: "Spread",
					Expr: &dag.This{Kind: "This", Path: path},
				})
			}
		} else {
			elems = append(elems, &dag.Field{
				Kind:  "Field",
				Name:  col.name,
				Value: col.scalar,
			})
		}
		e := &dag.RecordExpr{
			Kind: "RecordExpr",
			Elems: []dag.RecordElem{
				&dag.Field{
					Kind:  "Field",
					Name:  "in",
					Value: &dag.This{Kind: "This", Path: field.Path{"in"}},
				},
				&dag.Field{
					Kind: "Field",
					Name: "out",
					Value: &dag.RecordExpr{
						Kind:  "RecordExpr",
						Elems: elems,
					},
				},
			},
		}
		// {in:this,out:{a:e1,b:e2}}
		// | yield {in:this} (above)
		//
		// | yield {in,out:{a:e1}}
		// | yield {in,out:{...out,b:e2}}
		seq = append(seq, &dag.Yield{
			Kind:  "Yield",
			Exprs: []dag.Expr{e},
		})
	}
	return seq
}

func unravel(schema schema, prefix field.Path) []field.Path {
	switch schema := schema.(type) {
	default:
		return []field.Path{prefix}
	case *schemaSelect:
		return unravel(schema.in, append(prefix, "in"))
	case *schemaJoin:
		out := unravel(schema.left, append(prefix, "left"))
		return append(out, unravel(schema.right, append(prefix, "right"))...)
	}
}

//XXX need to detect mixed aggfunc calls with scalars that aren't
// in the input to an aggfunc... two traversals?  but we should let
// path refs to aggfunc results or group-by keys in...
// so traversal should detect scalar terms that aren't in group-by
// (or if group-by clause not present or we have group-by all we can
// infer they are group-by keys)

//XXX come up for tests for all these cases

func (a *aggExpr) build(colno int, e dag.Expr) dag.Expr {
	switch e := e.(type) {
	case nil:
		return e
	case *dag.Agg:
		//XXX don't do anything to input param or where clause
		// XXX need tests where we gen refs to non-existent things because
		// of scoping, e.g., select max(x) m, min(y) / m
		off := len(a.aggs)
		a.aggs = append(a.aggs, e)
		return pathOf(a.TempName(colno, off))
	case *dag.ArrayExpr:
		for _, elem := range e.Elems {
			switch elem := elem.(type) {
			case *dag.Spread:
				elem.Expr = a.build(colno, elem.Expr)
			case *dag.VectorValue:
				elem.Expr = a.build(colno, elem.Expr)
			default:
				panic(elem)
			}
		}
	case *dag.BinaryExpr:
		e.LHS = a.build(colno, e.LHS)
		e.RHS = a.build(colno, e.RHS)
	case *dag.Call:
		for k, arg := range e.Args {
			e.Args[k] = a.build(colno, arg)
		}
	case *dag.Conditional:
		e.Cond = a.build(colno, e.Cond)
		e.Then = a.build(colno, e.Then)
		e.Else = a.build(colno, e.Else)
	case *dag.Dot:
		e.LHS = a.build(colno, e.LHS)
	case *dag.Func:
		// XXX
	case *dag.IndexExpr:
		e.Expr = a.build(colno, e.Expr)
		e.Index = a.build(colno, e.Index)
	case *dag.IsNullExpr:
		e.Expr = a.build(colno, e.Expr)
	case *dag.Literal:
	case *dag.MapCall:
		e.Expr = a.build(colno, e.Expr)
	case *dag.MapExpr:
		for _, ent := range e.Entries {
			ent.Key = a.build(colno, ent.Key)
			ent.Value = a.build(colno, ent.Value)
		}
	case *dag.OverExpr:
		panic("TBD ERROR") //XXX
	case *dag.RecordExpr:
		for _, elem := range e.Elems {
			switch elem := elem.(type) {
			case *dag.Field:
				elem.Value = a.build(colno, elem.Value)
			case *dag.Spread:
				elem.Expr = a.build(colno, elem.Expr)
			default:
				panic(elem)
			}
		}
		return d
	case *dag.RegexpMatch:
		e.Expr = a.build(colno, e.Expr)
	case *dag.RegexpSearch:
		e.Expr = a.build(colno, e.Expr)
	case *dag.Search:
		e.Expr = a.build(colno, e.Expr)
	case *dag.SetExpr:
		for _, elem := range e.Elems {
			switch elem := elem.(type) {
			case *dag.Spread:
				elem.Expr = a.build(colno, elem.Expr)
			case *dag.VectorValue:
				elem.Expr = a.build(colno, elem.Expr)
			default:
				panic(elem)
			}
		}
	case *dag.SliceExpr:
		e.Expr = a.build(colno, e.Expr)
		e.From = a.build(colno, e.From)
		e.To = a.build(colno, e.To)
	case *dag.This:
	case *dag.UnaryExpr:
		e.Operand = a.build(colno, e.Operand)
	case *dag.Var:
	}
	return e
}
