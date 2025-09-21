package semantic

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/semantic/sem"
	"github.com/brimdata/super/pkg/field"
)

type schema interface {
	Name() string
	resolveColumn(col string) (field.Path, error)
	resolveOrdinal(n ast.Node, colno int) (sem.Expr, error)
	resolveTable(table string) (schema, field.Path, error)
	deref(n ast.Node, name string) (sem.Expr, schema)
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
	return sch, path, err
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

func (*dynamicSchema) resolveOrdinal(n ast.Node, col int) (sem.Expr, error) {
	if col <= 0 {
		return nil, fmt.Errorf("position %d is not in select list", col)
	}
	return &sem.IndexExpr{
		Node:  n,
		Expr:  sem.NewThis(n, nil),
		Index: &sem.LiteralExpr{Node: n, Value: strconv.Itoa(col)},
	}, nil
}

func (s *staticSchema) resolveOrdinal(n ast.Node, col int) (sem.Expr, error) {
	if col <= 0 || col > len(s.columns) {
		return nil, fmt.Errorf("position %d is not in select list", col)
	}
	return sem.NewThis(n, []string{s.columns[col-1]}), nil
}

func (s *selectSchema) resolveOrdinal(n ast.Node, col int) (sem.Expr, error) {
	if s.out != nil {
		if resolved, err := s.out.resolveOrdinal(n, col); resolved != nil {
			return appendExprToPath("out", resolved), nil
		} else if err != nil {
			return nil, err
		}
	}
	resolved, err := s.in.resolveOrdinal(n, col)
	if resolved != nil {
		return appendExprToPath("in", resolved), nil
	}
	return nil, err
}

func (j *joinSchema) resolveOrdinal(ast.Node, int) (sem.Expr, error) {
	return nil, errors.New("ordinal column selection in join not supported")
}

func (j *joinUsingSchema) resolveOrdinal(ast.Node, int) (sem.Expr, error) {
	return nil, errors.New("ordinal column selection in join not supported")
}

func (s *subquerySchema) resolveOrdinal(ast.Node, int) (sem.Expr, error) {
	return nil, errors.New("ordinal column selection in subquery not supported")
}

func appendExprToPath(path string, e sem.Expr) sem.Expr {
	switch e := e.(type) {
	case *sem.ThisExpr:
		return sem.NewThis(e, append([]string{path}, e.Path...))
	case *sem.IndexExpr:
		return &sem.IndexExpr{
			Node:  e,
			Expr:  appendExprToPath(path, e.Expr),
			Index: e.Index,
		}
	default:
		panic(e)
	}
}

func (d *dynamicSchema) deref(n ast.Node, name string) (sem.Expr, schema) {
	if name != "" {
		d = &dynamicSchema{name: name}
	}
	return nil, d
}

func (s *staticSchema) deref(n ast.Node, name string) (sem.Expr, schema) {
	if name != "" {
		s = &staticSchema{name: name, columns: s.columns}
	}
	return nil, s
}

func (s *selectSchema) deref(n ast.Node, name string) (sem.Expr, schema) {
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
	return sem.NewThis(n, []string{"out"}), outSchema
}

func (j *joinSchema) deref(n ast.Node, name string) (sem.Expr, schema) {
	// spread left/right join legs into "this"
	return joinSpread(n, nil, nil), &dynamicSchema{name: name}
}

// spread left/right join legs into "this"
func joinSpread(n ast.Node, left, right sem.Expr) *sem.RecordExpr {
	if left == nil {
		left = sem.NewThis(n, nil)
	}
	if right == nil {
		right = sem.NewThis(n, nil)
	}
	return &sem.RecordExpr{
		Node: n,
		Elems: []sem.RecordElem{
			&sem.SpreadElem{
				Node: n,
				Expr: left,
			},
			&sem.SpreadElem{
				Node: n,
				Expr: right,
			},
		},
	}
}

func (s *subquerySchema) deref(n ast.Node, name string) (sem.Expr, schema) {
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
