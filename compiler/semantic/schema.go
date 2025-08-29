package semantic

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/pkg/field"
)

type schema interface {
	Name() string
	resolveColumn(col string) (field.Path, error)
	resolveOrdinal(colno int) (dag.Expr, error)
	resolveTable(table string) (schema, field.Path, error)
	deref(name string) (dag.Expr, schema)
	String() string
}

type staticSchema struct {
	name    string
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

type joinUsingSchema struct {
	*joinSchema
}

type subquerySchema struct {
	outer schema
	inner schema
}

func (s *staticSchema) Name() string  { return s.name }
func (d *dynamicSchema) Name() string { return d.name }
func (*selectSchema) Name() string    { return "" }
func (*joinSchema) Name() string      { return "" }
func (*subquerySchema) Name() string  { return "" }

func badSchema() schema {
	return &dynamicSchema{}
}

func (d *dynamicSchema) resolveTable(table string) (schema, field.Path, error) {
	if table == "" || strings.EqualFold(d.name, table) {
		return d, nil, nil
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

func (j *joinUsingSchema) resolveTable(string) (schema, field.Path, error) {
	return nil, nil, fmt.Errorf("table selection in USING clause not allowed")
}

func (s *subquerySchema) resolveTable(table string) (schema, field.Path, error) {
	if table == "" {
		return s, nil, nil
	}
	sch, path, err := s.inner.resolveTable(table)
	if err != nil || sch != nil {
		return sch, path, err
	}
	sch, path, err = s.outer.resolveTable(table)
	if sch != nil {
		return nil, nil, errors.New("correlated subqueries not currently supported")
	}
	return nil, nil, err
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

func (j *joinUsingSchema) resolveColumn(col string) (field.Path, error) {
	if _, err := j.left.resolveColumn(col); err != nil {
		return nil, fmt.Errorf("column %q in USING clause does not exist in left table", col)
	}
	if _, err := j.right.resolveColumn(col); err != nil {
		return nil, fmt.Errorf("column %q in USING clause does not exist in right table", col)
	}
	return field.Path{col}, nil
}

func (s *subquerySchema) resolveColumn(col string) (field.Path, error) {
	path, _ := s.inner.resolveColumn(col)
	if path != nil {
		return path, nil
	}
	path, _ = s.outer.resolveColumn(col)
	if path != nil {
		return nil, errors.New("correlated subqueries not currently supported")
	}
	return nil, fmt.Errorf("column %q not found", col)
}

func (*dynamicSchema) resolveOrdinal(col int) (dag.Expr, error) {
	if col <= 0 {
		return nil, fmt.Errorf("position %d is not in select list", col)
	}
	return &dag.IndexExpr{
		Kind:  "IndexExpr",
		Expr:  dag.NewThis(nil),
		Index: &dag.Literal{Kind: "Literal", Value: strconv.Itoa(col)},
	}, nil
}

func (s *staticSchema) resolveOrdinal(col int) (dag.Expr, error) {
	if col <= 0 || col > len(s.columns) {
		return nil, fmt.Errorf("position %d is not in select list", col)
	}
	return dag.NewThis([]string{s.columns[col-1]}), nil
}

func (s *selectSchema) resolveOrdinal(col int) (dag.Expr, error) {
	if s.out != nil {
		if resolved, err := s.out.resolveOrdinal(col); resolved != nil {
			return appendExprToPath("out", resolved), nil
		} else if err != nil {
			return nil, err
		}
	}
	resolved, err := s.in.resolveOrdinal(col)
	if resolved != nil {
		return appendExprToPath("in", resolved), nil
	}
	return nil, err
}

func (j *joinSchema) resolveOrdinal(col int) (dag.Expr, error) {
	return nil, errors.New("ordinal column selection in join not supported")
}

func (j *joinUsingSchema) resolveOrdinal(col int) (dag.Expr, error) {
	return nil, errors.New("ordinal column selection in join not supported")
}

func (s *subquerySchema) resolveOrdinal(col int) (dag.Expr, error) {
	return nil, errors.New("ordinal column selection in subquery not supported")
}

func appendExprToPath(path string, e dag.Expr) dag.Expr {
	switch e := e.(type) {
	case *dag.This:
		return dag.NewThis(append([]string{path}, e.Path...))
	case *dag.IndexExpr:
		return &dag.IndexExpr{
			Kind:  "IndexExpr",
			Expr:  appendExprToPath(path, e.Expr),
			Index: e.Index,
		}
	default:
		panic(e)
	}
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

func (s *selectSchema) deref(name string) (dag.Expr, schema) {
	if name == "" {
		// postgres and duckdb oddly do this
		name = "unamed_subquery"
	}
	var outSchema schema
	if sch, ok := s.out.(*staticSchema); ok {
		// Hide any enclosing schema hierarchy by just exporting the
		// select columns.
		outSchema = &staticSchema{name: name, columns: sch.columns}
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
		left = dag.NewThis(nil)
	}
	if right == nil {
		right = dag.NewThis(nil)
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

func (s *subquerySchema) deref(name string) (dag.Expr, schema) {
	panic(name)
}

func (s *staticSchema) String() string {
	return fmt.Sprintf("static <%s>: %s", s.name, strings.Join(s.columns, ", "))
}

func (d *dynamicSchema) String() string {
	return fmt.Sprintf("dynamic <%s>", d.name)
}

func (s *selectSchema) String() string {
	return fmt.Sprintf("select:\n  in: %s\n  out: %s", s.in, s.out)
}

func (s *joinSchema) String() string {
	return fmt.Sprintf("join:\n  left: %s\n  right: %s", s.left, s.right)
}

func (s *subquerySchema) String() string {
	return fmt.Sprintf("subquery:\n  outer: %s\n  inner: %s", s.outer, s.inner)
}
