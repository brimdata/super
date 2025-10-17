package semantic

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/semantic/sem"
	"github.com/brimdata/super/pkg/field"
)

type schema interface {
	resolveColumn(col string) (field.Path, bool, error)
	resolveOrdinal(n ast.Node, colno int) (sem.Expr, error)
	resolveTable(table string) (schema, field.Path, error)
	unfurl(n ast.Node) sem.Expr
	endScope(n ast.Node) (sem.Expr, schema)
	this(n ast.Node, path []string) sem.Expr
	tableOnly(n ast.Node, table string, path []string) (sem.Expr, error)
	String() string
}

type staticSchema struct {
	columns []string
}

type dynamicSchema struct{}

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

type aliasSchema struct {
	name string
	sch  schema
}

func newSchemaFromType(typ super.Type) schema {
	if recType := recordOf(typ); recType != nil {
		var cols []string
		for _, f := range recType.Fields {
			cols = append(cols, f.Name)
		}
		return &staticSchema{cols}
	}
	return &dynamicSchema{}
}

func recordOf(typ super.Type) *super.TypeRecord {
	switch typ := super.TypeUnder(typ).(type) {
	case *super.TypeRecord:
		return typ
	case *super.TypeUnion:
		if hasUnknown(typ) {
			return nil
		}
		for _, typ := range typ.Types {
			if typ := recordOf(typ); typ != nil {
				return typ
			}
		}
	}
	return nil
}

func badSchema() schema {
	return &dynamicSchema{}
}

func (a *aliasSchema) resolveTable(table string) (schema, field.Path, error) {
	if strings.EqualFold(a.name, table) {
		return a.sch, nil, nil
	}
	return nil, nil, nil
}

func (d *dynamicSchema) resolveTable(table string) (schema, field.Path, error) {
	return nil, nil, nil
}

func (s *staticSchema) resolveTable(table string) (schema, field.Path, error) {
	return nil, nil, nil
}

func (s *selectSchema) resolveTable(table string) (schema, field.Path, error) {
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

func (a *aliasSchema) resolveColumn(col string) (field.Path, bool, error) {
	if a.name == col {
		// Special case the dynamic schema referencing a table alias without
		// any fields.  We declare this an error so that the same query with
		// schema information will not change it's result and favor a column
		// present with the same name as the table.  Instead of referring to
		// the table alias of a dynamic schema, the query could use `this` instead,
		// or a pipe query!
		if _, ok := a.sch.(*dynamicSchema); ok {
			return nil, true, fmt.Errorf("cannot reference column %q of dynamic input whose alias is %q (consider 'this')", col, a.name)
		}
	}
	return a.sch.resolveColumn(col)
}

func (d *dynamicSchema) resolveColumn(col string) (field.Path, bool, error) {
	return field.Path{col}, false, nil
}

func (s *staticSchema) resolveColumn(col string) (field.Path, bool, error) {
	if slices.Contains(s.columns, col) {
		return field.Path{col}, false, nil
	}
	return nil, false, fmt.Errorf("column %q: does not exist", col)
}

func (s *selectSchema) resolveColumn(col string) (field.Path, bool, error) {
	if s.out != nil {
		resolved, fatal, err := s.out.resolveColumn(col)
		if fatal {
			return nil, true, err
		}
		if resolved != nil {
			return append([]string{"out"}, resolved...), false, nil
		}
	}
	resolved, fatal, err := s.in.resolveColumn(col)
	if resolved != nil {
		return append([]string{"in"}, resolved...), false, nil
	}
	return nil, fatal, err
}

func (j *joinSchema) resolveColumn(col string) (field.Path, bool, error) {
	left, fatal, lerr := j.left.resolveColumn(col)
	if fatal {
		return nil, true, lerr
	}
	if left != nil {
		chk, fatal, err := j.right.resolveColumn(col)
		if fatal {
			return nil, true, err
		}
		if chk != nil {
			return nil, true, fmt.Errorf("%q: ambiguous column reference", col)
		}
		return append([]string{"left"}, left...), false, nil
	}
	if lerr == nil {
		// This shouldn't happen because the resolve return values should
		// always be nil/err or val/nil.
		panic("issue encountered in SQL name resolution")
	}
	right, fatal, rerr := j.right.resolveColumn(col)
	if fatal {
		return nil, true, rerr
	}
	if right != nil {
		return append([]string{"right"}, right...), false, nil
	}
	return nil, false, fmt.Errorf("%q: not found (%w, %w)", col, lerr, rerr)
}

func (j *joinUsingSchema) resolveColumn(col string) (field.Path, bool, error) {
	if _, _, err := j.left.resolveColumn(col); err != nil {
		return nil, false, fmt.Errorf("column %q in USING clause does not exist in left table", col)
	}
	if _, _, err := j.right.resolveColumn(col); err != nil {
		return nil, false, fmt.Errorf("column %q in USING clause does not exist in right table", col)
	}
	return field.Path{col}, false, nil
}

func (s *subquerySchema) resolveColumn(col string) (field.Path, bool, error) {
	path, fatal, err := s.inner.resolveColumn(col)
	if fatal {
		return nil, true, err
	}
	if path != nil {
		return path, false, nil
	}
	path, fatal, _ = s.outer.resolveColumn(col)
	if fatal {
		return nil, true, err
	}
	if path != nil {
		return nil, true, errors.New("correlated subqueries not currently supported")
	}
	return nil, false, fmt.Errorf("column %q not found", col)
}

func (a *aliasSchema) resolveOrdinal(n ast.Node, col int) (sem.Expr, error) {
	return a.sch.resolveOrdinal(n, col)
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

func (a *aliasSchema) unfurl(n ast.Node) sem.Expr {
	return a.sch.unfurl(n)
}

func (d *dynamicSchema) unfurl(n ast.Node) sem.Expr {
	return nil
}

func (s *staticSchema) unfurl(n ast.Node) sem.Expr {
	return nil
}

func (s *selectSchema) unfurl(n ast.Node) sem.Expr {
	return sem.NewThis(n, []string{"out"})
}

func (j *joinSchema) unfurl(n ast.Node) sem.Expr {
	// spread left/right join legs into "this"
	return joinSpread(n, nil, nil)
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

func (s *subquerySchema) unfurl(n ast.Node) sem.Expr {
	panic(s)
}

func (a *aliasSchema) endScope(n ast.Node) (sem.Expr, schema) {
	return a.sch.endScope(n)
}

func (d *dynamicSchema) endScope(n ast.Node) (sem.Expr, schema) {
	return nil, d
}

func (s *staticSchema) endScope(n ast.Node) (sem.Expr, schema) {
	return nil, s
}

func (s *selectSchema) endScope(n ast.Node) (sem.Expr, schema) {
	return sem.NewThis(n, []string{"out"}), s.out
}

func (j *joinSchema) endScope(n ast.Node) (sem.Expr, schema) {
	return nil, j
}

func (s *subquerySchema) endScope(n ast.Node) (sem.Expr, schema) {
	panic(s)
}

func (a *aliasSchema) this(n ast.Node, path []string) sem.Expr {
	return a.sch.this(n, path)
}

func (d *dynamicSchema) this(n ast.Node, path []string) sem.Expr {
	return sem.NewThis(n, path)
}

func (s *staticSchema) this(n ast.Node, path []string) sem.Expr {
	return sem.NewThis(n, path)
}

func (s *selectSchema) this(n ast.Node, path []string) sem.Expr {
	return s.in.this(n, append(path, "in"))
}

func (j *joinSchema) this(n ast.Node, path []string) sem.Expr {
	left := j.left.this(n, append(path, "left"))
	right := j.right.this(n, append(path, "right"))
	return joinSpread(n, left, right)
}

func (s *subquerySchema) this(n ast.Node, path []string) sem.Expr {
	panic("TBD")
}

func (a *aliasSchema) tableOnly(n ast.Node, name string, path []string) (sem.Expr, error) {
	if a.name == name {
		return a.sch.this(n, path), nil
	}
	return nil, fmt.Errorf("no such table %q", name)
}

func (d *dynamicSchema) tableOnly(n ast.Node, name string, path []string) (sem.Expr, error) {
	return nil, fmt.Errorf("illegal reference to table %q in dynamic input", name)
}

func (s *staticSchema) tableOnly(n ast.Node, name string, path []string) (sem.Expr, error) {
	return nil, fmt.Errorf("no such table %q", name)
}

func (s *selectSchema) tableOnly(n ast.Node, name string, path []string) (sem.Expr, error) {
	return s.in.tableOnly(n, name, append(path, "in"))
}

func (j *joinSchema) tableOnly(n ast.Node, name string, path []string) (sem.Expr, error) {
	e, err := j.left.tableOnly(n, name, append(path, "left"))
	if err != nil {
		return nil, err
	}
	if e != nil {
		ambig, _ := j.right.tableOnly(n, name, append(path, "right"))
		if ambig != nil {
			return nil, fmt.Errorf("ambiguous table reference %q", name)
		}
		return e, nil
	}
	return j.right.tableOnly(n, name, append(path, "right"))
}

func (s *subquerySchema) tableOnly(n ast.Node, name string, path []string) (sem.Expr, error) {
	return nil, fmt.Errorf("no such table %q", name) //XXX
}

func (a *aliasSchema) String() string {
	return fmt.Sprintf("alias <%s>", a.name)
}

func (s *staticSchema) String() string {
	return fmt.Sprintf("static %s", strings.Join(s.columns, ", "))
}

func (d *dynamicSchema) String() string {
	return "dynamic"
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

type havingSchema struct {
	*selectSchema
}

func (h *havingSchema) resolveColumn(col string) (field.Path, bool, error) {
	resolved, fatal, err := h.out.resolveColumn(col)
	if resolved != nil {
		return append([]string{"out"}, resolved...), false, nil
	}
	return nil, fatal, err
}

func (h *havingSchema) this(n ast.Node, path []string) sem.Expr {
	return h.out.this(n, append(path, "out"))
}

func (h *havingSchema) tableOnly(n ast.Node, name string, path []string) (sem.Expr, error) {
	return h.out.tableOnly(n, name, append(path, "out"))
}

func addAlias(sch schema, alias string) schema {
	switch sch := sch.(type) {
	case *aliasSchema:
		return &aliasSchema{
			name: alias,
			sch:  sch.sch,
		}
	case *dynamicSchema, *selectSchema, *staticSchema:
		return &aliasSchema{
			name: alias,
			sch:  sch,
		}
	default:
		panic(sch)
	}
}
