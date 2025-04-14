package semantic

import (
	"fmt"
	"slices"
	"strings"

	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/pkg/field"
)

type schema interface {
	Name() string
	resolveColumn(col string) (field.Path, error)
	resolveTable(table string) (schema, field.Path, error)
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
	if slices.Contains(s.columns, col) {
		return field.Path{col}, nil
	}
	return nil, fmt.Errorf("column %q: does not exist", col)
}

func (a *anonSchema) resolveColumn(col string) (field.Path, error) {
	if slices.Contains(a.columns, col) {
		return field.Path{col}, nil
	}
	return nil, fmt.Errorf("column %q: does not exist", col)
}

func (s *selectSchema) resolveColumn(col string) (field.Path, error) {
	if s.out != nil {
		if resolved, _ := s.out.resolveColumn(col); resolved != nil {
			return append([]string{"out"}, resolved...), nil
		}
	}
	resolved, err := s.in.resolveColumn(col)
	if resolved != nil {
		return append([]string{"in"}, resolved...), nil
	}
	return nil, err
}

func (j *joinSchema) resolveColumn(col string) (field.Path, error) {
	left, lerr := j.left.resolveColumn(col)
	if left != nil {
		if chk, _ := j.right.resolveColumn(col); chk != nil {
			return nil, fmt.Errorf("%q: ambiguous column reference", col)
		}
		return append([]string{"left"}, left...), nil
	}
	if lerr == nil {
		// This shouldn't happen because the resolve return values should
		// always be nil/err or val/nil.
		panic("issue encountered in SQL name resolution")
	}
	right, rerr := j.right.resolveColumn(col)
	if right != nil {
		return append([]string{"right"}, right...), nil
	}
	return nil, fmt.Errorf("%q: not found (%w, %w)", col, lerr, rerr)
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
