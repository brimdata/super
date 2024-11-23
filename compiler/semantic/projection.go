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
	name   string
	column []string
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

func (p projection) yieldScalars(seq dag.Seq) dag.Seq {
	if len(p) == 0 {
		return nil
	}
	var elems []dag.RecordElem
	for _, col := range p {
		var elem dag.RecordElem
		if col.isStar() {
			elem = &dag.Spread{
				Kind: "Spread",
				Expr: &dag.This{Kind: "This"},
			}
		} else {
			elem = &dag.Field{
				Kind:  "Field",
				Name:  col.name,
				Value: col.scalar,
			}
		}
		elems = append(elems, elem)
	}
	return append(seq, &dag.Yield{
		Kind: "Yield",
		Exprs: []dag.Expr{
			&dag.RecordExpr{
				Kind:  "RecordExpr",
				Elems: elems,
			},
		},
	})
}
