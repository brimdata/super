package semantic

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/semantic/sem"
	"github.com/brimdata/super/pkg/field"
)

type schema interface {
	resolveQualified(table, col string) (field.Path, error)
	resolveUnqualified(col string) (field.Path, bool, error)
	resolveTable(n ast.Node, table string, path []string) (sem.Expr, bool, error)
	star(n ast.Node, table string, path field.Path) ([]*sem.ThisExpr, error)
	unfurl(n ast.Node) sem.Expr
	endScope(n ast.Node, seq sem.Seq) (sem.Seq, schema)
	this(n ast.Node, path []string) sem.Expr
	outColumns() ([]string, bool)
	String() string
}

type (
	dynamicSchema struct {
		table string
	}

	joinSchema struct {
		left  schema
		right schema
	}

	// joinUsingSchema specializes the joinSchema so that * expansion
	// follows the ANSI standard by eliminating the duplicate using columns
	// from both sides and putting those columns in the leftmost positions.
	// The semantic logic checks that each column is present in both sides
	// of the join so there is no need to do that here and the unqualified
	// method matches the left side arbitrarily.
	joinUsingSchema struct {
		*joinSchema
		// The using condition columns
		columns []string
		skip    map[string]struct{}
	}

	// Schema is...
	// {in:<input>,out:<projection>} for non-grouped
	// {g:{k0,k1,...,a0,a1,...},out:<projection>} for grouped
	selectSchema struct {
		in schema

		// aggOk set when it's ok to process agg functions in expressions
		aggOk bool

		// grouped is set when processing an expression that should resolve
		// "this" to the aggregate projection (i.e., "g" column)
		grouped bool

		// aggs is the set of agg functions computed in a grouped table
		// inclusive of agg functions used subsequently in an order by
		aggs []*sem.AggFunc

		// groupings holds the grouping expressions which are needed to do
		// expression matching for having, order by, and the projection.
		// These expressions are always relative to the "in" scope.
		// The grouped field true when resolution has access only to the outputs.
		groupings []exprloc

		// groupByLoc is set when invoking t.expr() from an ordinal reference
		// so that we can produce a more understandable error message.
		groupByLoc ast.Node

		// Columns holds the projection.  We bookkeep both the ast expression
		// and sem tree expr so we can do lateral column alias macro substition.
		// The lateral field is true when lateral column resolution is allowed
		// during the analysis GROUP BY expressions.
		columns []column
		lateral bool

		// The output schema is always static, e.g., a dynamic input is
		// always wrapped in a column.  Also, the projected columns referenced
		// in ORDER BY have precedence over the rest of the SELECT expressions
		// (in contrast to the rest of the SELECT body, where lateral column
		// aliases have lwoer precedecne).
		out *staticSchema
	}

	staticSchema struct {
		table   string
		columns []string
	}

	subquerySchema struct {
		outer schema
		inner schema
	}
)

var badType = &super.TypeOfNull{}

func newSchemaFromType(typ super.Type) schema {
	if typ == badType {
		return badSchema
	}
	if recType := recordOf(typ); recType != nil {
		var cols []string
		for _, f := range recType.Fields {
			cols = append(cols, f.Name)
		}
		return &staticSchema{columns: cols}
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
		// We assume here this is a fused type and thus has only one record
		// type in the union.
		for _, typ := range typ.Types {
			if typ := recordOf(typ); typ != nil {
				return typ
			}
		}
	}
	return nil
}

var badSchema = &dynamicSchema{}

func newJoinUsingSchema(j *joinSchema, columns []string) *joinUsingSchema {
	skip := make(map[string]struct{})
	for _, c := range columns {
		skip[c] = struct{}{}
	}
	return &joinUsingSchema{
		joinSchema: j,
		columns:    columns,
		skip:       skip,
	}

}

func (s *selectSchema) isGrouped() bool {
	return len(s.aggs) != 0 || len(s.groupings) != 0
}

func (d *dynamicSchema) resolveQualified(table, col string) (field.Path, error) {
	if d.table == table {
		return field.Path{col}, nil
	}
	return nil, nil
}

func (j *joinSchema) resolveQualified(table, col string) (field.Path, error) {
	left, err := j.left.resolveQualified(table, col)
	if err != nil {
		return nil, err
	}
	if left != nil {
		chk, err := j.right.resolveQualified(table, col)
		if err != nil {
			return nil, err
		}
		if chk != nil {
			return nil, fmt.Errorf("ambiguous qualified reference %q.%q", table, col)
		}
		return append([]string{"left"}, left...), nil
	}
	right, err := j.right.resolveQualified(table, col)
	if err != nil {
		return nil, err
	}
	if right != nil {
		return append([]string{"right"}, right...), nil
	}
	return nil, nil
}

func (s *selectSchema) resolveQualified(table, col string) (field.Path, error) {
	path, err := s.in.resolveQualified(table, col)
	if path != nil {
		return append([]string{"in"}, path...), nil
	}
	return nil, err
}

func (s *staticSchema) resolveQualified(table, col string) (field.Path, error) {
	if s.table == table && slices.Contains(s.columns, col) {
		return field.Path{col}, nil
	}
	return nil, nil
}

func (s *subquerySchema) resolveQualified(table, col string) (field.Path, error) {
	path, err := s.inner.resolveQualified(table, col)
	if err != nil {
		return nil, err
	}
	if path != nil {
		return path, nil
	}
	path, err = s.outer.resolveQualified(table, col)
	if err != nil {
		return nil, err
	}
	if path != nil {
		return nil, errors.New("correlated subqueries not currently supported")
	}
	return nil, nil
}

func (d *dynamicSchema) resolveUnqualified(col string) (field.Path, bool, error) {
	return field.Path{col}, true, nil
}

func (j *joinSchema) resolveUnqualified(col string) (field.Path, bool, error) {
	left, ldyn, err := j.left.resolveUnqualified(col)
	if ldyn && err == nil {
		err = fmt.Errorf("join on dynamic column %q requires table-qualified reference", col)
	}
	if err != nil {
		return nil, ldyn, err
	}
	if left != nil {
		chk, rdyn, err := j.right.resolveUnqualified(col)
		if rdyn && err == nil {
			err = fmt.Errorf("join on dynamic column %q requires table-qualified reference", col)
		}
		if err != nil {
			return nil, rdyn, err
		}
		if chk != nil {
			return nil, false, fmt.Errorf("ambiguous unqualified column %q", col)
		}
		return append([]string{"left"}, left...), false, nil
	}
	right, rdyn, err := j.right.resolveUnqualified(col)
	if rdyn && err == nil {
		err = fmt.Errorf("join on dynamic column %q requires table-qualified reference", col)
	}
	if err != nil {
		return nil, rdyn, err
	}
	if right != nil {
		return append([]string{"right"}, right...), rdyn, nil
	}
	return nil, false, nil
}

func (j *joinUsingSchema) resolveUnqualified(col string) (field.Path, bool, error) {
	if slices.Contains(j.columns, col) {
		// avoid ambiguous column reference and return arbitrarily return left side
		left, dyn, err := j.left.resolveUnqualified(col)
		if err != nil {
			return nil, dyn, err
		}
		return append([]string{"left"}, left...), dyn, nil
	}
	// for non-using columns, ambiguous colomn references should be reported
	return j.joinSchema.resolveUnqualified(col)
}

func (s *selectSchema) resolveUnqualified(col string) (field.Path, bool, error) {
	// This just looks for column in the input table.  The resolve() function
	// looks at lateral column aliases if this fails.
	if s.out != nil && !s.isGrouped() {
		// The output scope is set after the select scope is almost closed so that
		// ORDER BY can utilize the projected columns, which have precedence
		// higher than anything else, except when recursively descending
		// into the argument expression of agg functions, in which case,
		// s.out is cleared and scalars resolve to input and lateral columns.
		path, dyn, err := s.out.resolveUnqualified(col)
		if err != nil {
			return nil, false, err
		}
		if path != nil {
			return append([]string{"out"}, path...), dyn, nil
		}
	}
	path, dyn, err := s.in.resolveUnqualified(col)
	if path != nil {
		return append([]string{"in"}, path...), dyn, nil
	}
	return nil, false, err
}

func (s *staticSchema) resolveUnqualified(col string) (field.Path, bool, error) {
	if slices.Contains(s.columns, col) {
		return field.Path{col}, false, nil
	}
	return nil, false, nil
}

func (s *subquerySchema) resolveUnqualified(col string) (field.Path, bool, error) {
	path, dyn, err := s.inner.resolveUnqualified(col)
	if err != nil {
		return nil, dyn, err
	}
	if path != nil {
		return path, dyn, nil
	}
	path, dyn, err = s.outer.resolveUnqualified(col)
	if err != nil {
		return nil, dyn, err
	}
	if path != nil {
		return nil, dyn, errors.New("correlated subqueries not currently supported")
	}
	return nil, false, nil
}

func (d *dynamicSchema) star(n ast.Node, table string, path field.Path) ([]*sem.ThisExpr, error) {
	return nil, errors.New("the all-columns (*) pattern cannot be used for dynamic inputs")
}

func (j *joinSchema) star(n ast.Node, table string, path field.Path) ([]*sem.ThisExpr, error) {
	left, err := j.left.star(n, table, append(path, "left"))
	if err != nil {
		return nil, err
	}
	right, err := j.right.star(n, table, append(path, "right"))
	if err != nil {
		return nil, err
	}
	return append(left, right...), nil
}

func (j *joinUsingSchema) star(n ast.Node, table string, path field.Path) ([]*sem.ThisExpr, error) {
	if table != "" {
		// If this is a table.* match, then we don't need to worry about putting
		// the USING columns on the left.  Just let the underlying join do the work.
		return j.joinSchema.star(n, table, path)
	}
	// ANSI SQL says the USING columns are leftmost in the table, then the remaining.
	var out []*sem.ThisExpr
	for _, col := range j.columns {
		left, _, err := j.left.resolveUnqualified(col)
		if err != nil {
			return nil, err
		}
		p := append(append(path, "left"), left...)
		this := sem.NewThis(n, p)
		out = append(out, this)
	}
	var err error
	out, err = j.filterAppend(out, j.left, n, append(path, "left"))
	if err != nil {
		return nil, err
	}
	return j.filterAppend(out, j.right, n, append(path, "right"))
}

func (j *joinUsingSchema) filterAppend(out []*sem.ThisExpr, s schema, n ast.Node, path []string) ([]*sem.ThisExpr, error) {
	exprs, err := s.star(n, "", path)
	if err != nil {
		return nil, err
	}
	for _, this := range exprs {
		if _, skip := j.skip[this.Path[len(this.Path)-1]]; !skip {
			out = append(out, this)
		}
	}
	return out, nil
}

func (s *selectSchema) star(n ast.Node, table string, path field.Path) ([]*sem.ThisExpr, error) {
	return s.in.star(n, table, append(path, "in"))
}

func (s *staticSchema) star(n ast.Node, table string, path field.Path) ([]*sem.ThisExpr, error) {
	var out []*sem.ThisExpr
	if table == "" || s.table == table {
		for _, col := range s.columns {
			out = append(out, sem.NewThis(n, append(path, col)))
		}
	}
	return out, nil
}

func (s *subquerySchema) star(n ast.Node, table string, path field.Path) ([]*sem.ThisExpr, error) {
	return s.inner.star(n, table, path)
}

func (d *dynamicSchema) unfurl(n ast.Node) sem.Expr {
	return nil
}

func (j *joinSchema) unfurl(n ast.Node) sem.Expr {
	// spread left/right join legs into "this"
	return joinSpread(n, nil, nil)
}

func (s *selectSchema) unfurl(n ast.Node) sem.Expr {
	return sem.NewThis(n, []string{"out"})
}

func (s *staticSchema) unfurl(n ast.Node) sem.Expr {
	return nil
}

func (s *subquerySchema) unfurl(n ast.Node) sem.Expr {
	panic(s)
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

func (d *dynamicSchema) endScope(n ast.Node, seq sem.Seq) (sem.Seq, schema) {
	return seq, d
}

func (j *joinSchema) endScope(n ast.Node, seq sem.Seq) (sem.Seq, schema) {
	return seq, j
}

func (s *selectSchema) endScope(n ast.Node, seq sem.Seq) (sem.Seq, schema) {
	var sch schema
	// s.out can be nil for error conditions and we need to avoid non-nil interface
	if s.out != nil {
		sch = s.out
	}
	return valuesExpr(sem.NewThis(n, []string{"out"}), seq), sch
}

func (s *staticSchema) endScope(n ast.Node, seq sem.Seq) (sem.Seq, schema) {
	return seq, s
}

func (s *subquerySchema) endScope(n ast.Node, seq sem.Seq) (sem.Seq, schema) {
	panic(s)
}

func (d *dynamicSchema) this(n ast.Node, path []string) sem.Expr {
	return sem.NewThis(n, path)
}

func (j *joinSchema) this(n ast.Node, path []string) sem.Expr {
	left := j.left.this(n, append(path, "left"))
	right := j.right.this(n, append(path, "right"))
	return joinSpread(n, left, right)
}

func (s *selectSchema) this(n ast.Node, path []string) sem.Expr {
	if s.grouped {
		return s.out.this(n, append(path, "out"))
	}
	return s.in.this(n, append(path, "in"))
}

func (s *staticSchema) this(n ast.Node, path []string) sem.Expr {
	return sem.NewThis(n, path)
}

func (s *subquerySchema) this(n ast.Node, path []string) sem.Expr {
	panic("TBD")
}

func (d *dynamicSchema) resolveTable(n ast.Node, table string, path []string) (sem.Expr, bool, error) {
	if d.table == table {
		// Can't refer to dynamic inputs by the table name because of the
		// table changes to static and has a column of that name, the query
		// semantics change.  Instead, consider "this".
		return nil, true, fmt.Errorf("illegal reference to table %q in dynamic input (consider using 'this')", table)
	}
	return nil, false, nil
}

func (j *joinSchema) resolveTable(n ast.Node, name string, path []string) (sem.Expr, bool, error) {
	e, dyn, err := j.left.resolveTable(n, name, append(path, "left"))
	if err != nil {
		return nil, dyn, err
	}
	if e != nil {
		ambig, _, _ := j.right.resolveTable(n, name, append(path, "right"))
		if ambig != nil {
			return nil, dyn, fmt.Errorf("ambiguous table reference %q", name)
		}
		return e, dyn, nil
	}
	return j.right.resolveTable(n, name, append(path, "right"))
}

func (s *selectSchema) resolveTable(n ast.Node, name string, path []string) (sem.Expr, bool, error) {
	return s.in.resolveTable(n, name, append(path, "in"))
}

func (s *staticSchema) resolveTable(n ast.Node, table string, path []string) (sem.Expr, bool, error) {
	if s.table == table {
		return sem.NewThis(n, path), false, nil
	}
	return nil, false, nil
}

func (s *subquerySchema) resolveTable(n ast.Node, table string, path []string) (sem.Expr, bool, error) {
	// XXX should search outer once we have suppport for correlated subq
	return s.inner.resolveTable(n, table, path)
}

func (d *dynamicSchema) outColumns() ([]string, bool) {
	return nil, false
}

func (j *joinSchema) outColumns() ([]string, bool) {
	left, lok := j.left.outColumns()
	right, rok := j.right.outColumns()
	if !lok || !rok {
		return nil, false
	}
	// Include all columns from the left except for those appearing on
	// the right since that's what {...left,...right} does.
	m := make(map[string]struct{})
	for _, col := range right {
		m[col] = struct{}{}
	}
	var out []string
	for _, col := range left {
		if _, ok := m[col]; !ok {
			out = append(out, col)
		}
	}
	return append(out, right...), true
}

func (s *selectSchema) outColumns() ([]string, bool) {
	return s.out.outColumns()
}

func (s *staticSchema) outColumns() ([]string, bool) {
	return s.columns, true
}

func (s *subquerySchema) outColumns() ([]string, bool) {
	return s.outer.outColumns()
}

func (d *dynamicSchema) String() string {
	return "dynamic"
}

func (s *joinSchema) String() string {
	return fmt.Sprintf("join:\n  left: %s\n  right: %s", s.left, s.right)
}

func (s *selectSchema) String() string {
	return fmt.Sprintf("select:\n  in: %s\n  out: %s", s.in, s.out)
}

func (s *staticSchema) String() string {
	return fmt.Sprintf("static %s", strings.Join(s.columns, ", "))
}

func (s *subquerySchema) String() string {
	return fmt.Sprintf("subquery:\n  outer: %s\n  inner: %s", s.outer, s.inner)
}

func addTableAlias(sch schema, table string) schema {
	switch sch := sch.(type) {
	case *dynamicSchema:
		return &dynamicSchema{table: table}
	case *staticSchema:
		return &staticSchema{
			table:   table,
			columns: sch.columns,
		}
	default:
		// Should never be trying to apply a table alias to the
		// other schema types.
		panic(sch)
	}
}
