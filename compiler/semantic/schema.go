package semantic

import (
	"fmt"
	"strings"

	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/pkg/field"
)

type schema interface {
	Name() string
	resolveColumn(col string) (field.Path, error)
	resolveTable(table string) (schema, field.Path, error)
	// deref adds logic to seq to yield out the value from a SQL-schema-contained
	// value set and returns the resulting schema.  If name is non-zero, then a new
	// schema is returned that represents the aliased table name that results.
	// XXX fix cmoment about name semantic
	deref(name string) (dag.Expr, schema)
}

type staticSchema struct {
	name    string
	columns []string
}

type anonSchema struct {
	columns []string
}

type dynamicSchema struct {
	name string
}

type selectSchema struct {
	in  schema
	out schema
}

type joinSchema struct {
	left  schema
	right schema
}

func (s *staticSchema) Name() string  { return s.name }
func (d *dynamicSchema) Name() string { return d.name }
func (*anonSchema) Name() string      { return "" }
func (*selectSchema) Name() string    { return "" }
func (*joinSchema) Name() string      { return "" }

func badSchema() schema {
	return &dynamicSchema{}
}

func (d *dynamicSchema) resolveTable(table string) (schema, field.Path, error) {
	if table == "" || strings.EqualFold(d.name, table) {
		return d, nil, nil
	}
	return nil, nil, nil
}

func (a *anonSchema) resolveTable(table string) (schema, field.Path, error) {
	if table == "" {
		return a, nil, nil
	}
	return nil, nil, nil
}

func (s *staticSchema) resolveTable(table string) (schema, field.Path, error) {
	if table == "" || strings.EqualFold(s.name, table) {
		return s, nil, nil
	}
	return nil, nil, nil
}

func (s *selectSchema) resolveTable(table string) (schema, field.Path, error) {
	if table == "" {
		sch, path, err := s.in.resolveTable(table)
		if sch != nil {
			path = append([]string{"in"}, path...)
		}
		return sch, path, err
	}
	if s.out != nil {
		sch, path, err := s.out.resolveTable(table)
		if err != nil {
			return nil, nil, err
		}
		if sch != nil {
			return sch, append([]string{"out"}, path...), nil
		}
	}
	sch, path, err := s.in.resolveTable(table)
	if err != nil {
		return nil, nil, err
	}
	if sch != nil {
		return sch, append([]string{"in"}, path...), nil
	}
	return nil, nil, nil
}

func (j *joinSchema) resolveTable(table string) (schema, field.Path, error) {
	if table == "" {
		return j, nil, nil
	}
	sch, path, err := j.left.resolveTable(table)
	if err != nil {
		return nil, nil, err
	}
	if sch != nil {
		chk, _, err := j.right.resolveTable(table)
		if err != nil {
			return nil, nil, err
		}
		if chk != nil {
			return nil, nil, fmt.Errorf("%q: ambiguous table reference", table)
		}
		return sch, append([]string{"left"}, path...), nil
	}
	sch, path, err = j.right.resolveTable(table)
	if sch == nil || err != nil {
		return nil, nil, err
	}
	return sch, append([]string{"right"}, path...), nil
}

func (*dynamicSchema) resolveColumn(col string) (field.Path, error) {
	return field.Path{col}, nil
}

func (s *staticSchema) resolveColumn(col string) (field.Path, error) {
	for _, c := range s.columns {
		if c == col {
			return field.Path{col}, nil
		}
	}
	return nil, nil
}

func (a *anonSchema) resolveColumn(col string) (field.Path, error) {
	for _, c := range a.columns {
		if c == col {
			return field.Path{col}, nil
		}
	}
	return nil, nil
}

func (s *selectSchema) resolveColumn(col string) (field.Path, error) {
	if s.out != nil {
		resolved, err := s.out.resolveColumn(col)
		if err != nil {
			return nil, err
		}
		if resolved != nil {
			return append([]string{"out"}, resolved...), nil
		}
	}
	resolved, err := s.in.resolveColumn(col)
	if err != nil {
		return nil, err
	}
	if resolved != nil {
		return append([]string{"in"}, resolved...), nil
	}
	return nil, nil
}

func (j *joinSchema) resolveColumn(col string) (field.Path, error) {
	out, err := j.left.resolveColumn(col)
	if err != nil {
		return nil, err
	}
	if out != nil {
		chk, err := j.right.resolveColumn(col)
		if err != nil {
			return nil, err
		}
		if chk != nil {
			return nil, fmt.Errorf("%q: ambiguous column reference", col)
		}
		return append([]string{"left"}, out...), nil
	}
	out, err = j.right.resolveColumn(col)
	if err != nil {
		return nil, err
	}
	if out != nil {
		return append([]string{"right"}, out...), nil
	}
	return nil, nil
}

func (d *dynamicSchema) deref(name string) (dag.Expr, schema) {
	if name != "" {
		d = &dynamicSchema{name: name}
	}
	return nil, d
}

func (s *staticSchema) deref(name string) (dag.Expr, schema) {
	if name != "" {
		s = &staticSchema{name: name, columns: s.columns}
	}
	return nil, s
}

func (a *anonSchema) deref(name string) (dag.Expr, schema) {
	return nil, a
}

func (s *selectSchema) deref(name string) (dag.Expr, schema) {
	if name == "" {
		// postgres and duckdb oddly do this
		name = "unamed_subquery"
	}
	var outSchema schema
	if anon, ok := s.out.(*anonSchema); ok {
		// Hide any enclosing schema hierarchy by just exporting the
		// select columns.
		outSchema = &staticSchema{name: name, columns: anon.columns}
	} else {
		// This is a select value.
		// XXX we should eventually have a way to propagate schema info here,
		// e.g., record expression with fixed columns as an anonSchema.
		outSchema = &dynamicSchema{name: name}
	}
	return pathOf("out"), outSchema
}

func (j *joinSchema) deref(name string) (dag.Expr, schema) {
	// spread left/right join legs into "this"
	return joinSpread(nil, nil), &dynamicSchema{name: name}
}

// spread left/right join legs into "this"
func joinSpread(left, right dag.Expr) *dag.RecordExpr {
	if left == nil {
		left = &dag.This{Kind: "This"}
	}
	if right == nil {
		right = &dag.This{Kind: "This"}
	}
	return &dag.RecordExpr{
		Kind: "RecordExpr",
		Elems: []dag.RecordElem{
			&dag.Spread{
				Kind: "Spread",
				Expr: left,
			},
			&dag.Spread{
				Kind: "Spread",
				Expr: right,
			},
		},
	}
}
