package semantic

import (
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/pkg/field"
)

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
type column struct {
	name   string
	agg    *dag.Agg
	scalar dag.Expr
}

func (c column) isStar() bool {
	return c.agg == nil && c.scalar == nil
}

func isStar(a ast.AsExpr) bool {
	return a.Expr == nil && a.ID == nil
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

// Return the scalar paths that are in the selection.
func (p projection) paths() field.List {
	var fields field.List
	for _, col := range p {
		if col.scalar != nil {
			if this, ok := col.scalar.(*dag.This); ok {
				fields = append(fields, this.Path)
			}
		}
	}
	return fields
}

func (p projection) aggs() projection {
	var aggs projection
	for _, col := range p {
		if col.agg != nil {
			aggs = append(aggs, col)
		}
	}
	return aggs
}

func (p projection) scalars() projection {
	var scalars projection
	for _, col := range p {
		if col.agg == nil {
			scalars = append(scalars, col)
		}
	}
	return scalars
}

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
