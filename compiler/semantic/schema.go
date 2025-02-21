package semantic

import (
	"fmt"
	"strings"

	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/pkg/field"
)

type schema interface {
	Name() string
	resolveColumn(col string, path field.Path) (field.Path, error)
	resolveTable(table string, path field.Path) (field.Path, error)
	// deref adds logic to seq to yield out the value from a SQL-schema-contained
	// value set and returns the resulting schema.  If name is non-zero, then a new
	// schema is returned that represents the aliased table name that results.
	deref(seq dag.Seq, name string) (dag.Seq, schema)
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

func (d *dynamicSchema) resolveTable(table string, path field.Path) (field.Path, error) {
	if strings.EqualFold(d.name, table) {
		return path, nil
	}
	return nil, nil
}

func (*anonSchema) resolveTable(table string, path field.Path) (field.Path, error) {
	return nil, nil
}

func (s *staticSchema) resolveTable(table string, path field.Path) (field.Path, error) {
	if strings.EqualFold(s.name, table) {
		if len(path) == 0 {
			return []string{}, nil
		}
		return s.resolveColumn(path[0], path[1:])
	}
	return nil, nil
}

func (s *selectSchema) resolveTable(table string, path field.Path) (field.Path, error) {
	if s.out != nil {
		target, err := s.out.resolveTable(table, path)
		if err != nil {
			return nil, err
		}
		if target != nil {
			return append([]string{"out"}, target...), nil
		}
	}
	target, err := s.in.resolveTable(table, path)
	if err != nil {
		return nil, err
	}
	if target != nil {
		return append([]string{"in"}, target...), nil
	}
	return nil, nil
}

func (j *joinSchema) resolveTable(table string, path field.Path) (field.Path, error) {
	out, err := j.left.resolveTable(table, path)
	if err != nil {
		return nil, err
	}
	if out != nil {
		chk, err := j.right.resolveTable(table, path)
		if err != nil {
			return nil, err
		}
		if chk != nil {
			return nil, fmt.Errorf("%q: ambiguous table reference", table)
		}
		return append([]string{"left"}, out...), nil
	}
	out, err = j.right.resolveTable(table, path)
	if err != nil {
		return nil, err
	}
	if out != nil {
		return append([]string{"right"}, out...), nil
	}
	return nil, nil
}

func (*dynamicSchema) resolveColumn(col string, path field.Path) (field.Path, error) {
	return append([]string{col}, path...), nil
}

func (s *staticSchema) resolveColumn(col string, path field.Path) (field.Path, error) {
	for _, c := range s.columns {
		if c == col {
			return append([]string{col}, path...), nil
		}
	}
	return nil, nil
}

func (a *anonSchema) resolveColumn(col string, path field.Path) (field.Path, error) {
	for _, c := range a.columns {
		if c == col {
			return append([]string{col}, path...), nil
		}
	}
	return nil, nil
}

func (s *selectSchema) resolveColumn(col string, path field.Path) (field.Path, error) {
	if s.out != nil {
		resolved, err := s.out.resolveColumn(col, path)
		if err != nil {
			return nil, err
		}
		if resolved != nil {
			return append([]string{"out"}, resolved...), nil
		}
	}
	resolved, err := s.in.resolveColumn(col, path)
	if err != nil {
		return nil, err
	}
	if resolved != nil {
		return append([]string{"in"}, resolved...), nil
	}
	return nil, nil
}

func (j *joinSchema) resolveColumn(col string, path field.Path) (field.Path, error) {
	out, err := j.left.resolveColumn(col, path)
	if err != nil {
		return nil, err
	}
	if out != nil {
		chk, err := j.right.resolveColumn(col, path)
		if err != nil {
			return nil, err
		}
		if chk != nil {
			return nil, fmt.Errorf("%q: ambiguous column reference", col)
		}
		return append([]string{"left"}, out...), nil
	}
	out, err = j.right.resolveColumn(col, path)
	if err != nil {
		return nil, err
	}
	if out != nil {
		return append([]string{"right"}, out...), nil
	}
	return nil, nil
}

func (d *dynamicSchema) deref(seq dag.Seq, name string) (dag.Seq, schema) {
	if name != "" {
		d = &dynamicSchema{name: name}
	}
	return seq, d
}

func (s *staticSchema) deref(seq dag.Seq, name string) (dag.Seq, schema) {
	if name != "" {
		s = &staticSchema{name: name, columns: s.columns}
	}
	return seq, s
}

func (a *anonSchema) deref(seq dag.Seq, name string) (dag.Seq, schema) {
	return seq, a
}

func (s *selectSchema) deref(seq dag.Seq, name string) (dag.Seq, schema) {
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
		outSchema = &dynamicSchema{}
	}
	return append(seq, &dag.Yield{
		Kind:  "Yield",
		Exprs: []dag.Expr{pathOf("out")},
	}), outSchema
}

func (j *joinSchema) deref(seq dag.Seq, name string) (dag.Seq, schema) {
	// spread left/right join legs into "this"
	e := &dag.RecordExpr{
		Kind: "RecordExpr",
		Elems: []dag.RecordElem{
			&dag.Spread{
				Kind: "Spread",
				Expr: &dag.This{Kind: "This", Path: []string{"left"}},
			},
			&dag.Spread{
				Kind: "Spread",
				Expr: &dag.This{Kind: "This", Path: []string{"right"}},
			},
		},
	}
	return append(seq, &dag.Yield{
		Kind:  "Yield",
		Exprs: []dag.Expr{e},
	}), &dynamicSchema{name: name}
}
