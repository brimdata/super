package semantic

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/compiler/semantic/sem"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/sam/expr/agg"
)

// Analyze a SQL select expression which may have arbitrary nested subqueries
// and may or may not have its sources embedded.
// The output of a select expression is a record that wraps its input and
// selected columns in a record {in:any,out:any}.  The schema returned represents
// the observable scope of the selected elements.  When the parent operator is
// an OrderBy, it can reach into the "in" part of the select scope (for non-aggregates)
// and also sort by the out elements.  It's up to the caller to unwrap the in/out
// record when returning to pipeline context.
func (t *translator) sqlSelect(sel *ast.SQLSelect, demand []ast.Expr, seq sem.Seq) (sem.Seq, schema) {
	if len(sel.Selection.Args) == 0 {
		t.error(sel, errors.New("SELECT clause has no selection"))
		return seq, badSchema
	}
	seq, fromSchema := t.selectFrom(sel.Loc, sel.From, seq)
	if fromSchema == nil || fromSchema == badSchema {
		return seq, badSchema
	}
	if t.scope.schema != nil {
		fromSchema = &subquerySchema{
			outer: t.scope.schema,
			inner: fromSchema,
		}
	}
	save := t.scope.schema
	sch := &selectSchema{
		in: fromSchema,
	}
	t.scope.schema = sch
	defer func() {
		t.scope.schema = save
	}()
	// form column slots (and expand *'s) so we can resolve lateral column aliases
	sch.columns = t.formProjection(sch, sel.Selection.Args)
	sch.latStop = len(sch.columns)
	seq = valuesExpr(wrapThis(sel, "in"), seq)
	if sel.Where != nil {
		where := t.expr(sel.Where)
		seq = append(seq, sem.NewFilter(sel.Where, where))
	}
	sch.groupings = t.groupBy(sch, sel.GroupBy)
	sch.aggOk = true
	// Make sure any aggs needed by ORDER BY and computed.
	for _, e := range demand {
		t.expr(e)
	}
	// Next analyze all the columns so we know if there are aggregate functions
	// (in particular, we know this when there is no GROUP BY) so after this,
	// we know for sure if the output is grouped or not.
	t.projection(sch, sch.columns)
	// We now stitch together the fragments into pipeline operators depending
	// on whether the ouput is grouped or not.
	var having sem.Expr
	if sch.isGrouped() {
		if sel.Having != nil {
			having = t.groupedExpr(sch, sel.Having)
		}
		// Match grouping expressions in the projected column expressions.
		t.replaceGroupings(sch, sch.columns)
		seq = t.genAggregate(sel.Loc, sch, seq)
		seq = valuesExpr(wrapThis(sel, "g"), seq)
	} else if sel.Having != nil {
		t.error(sel.Having, errors.New("HAVING clause requires aggregation functions and/or a GROUP BY clause"))
	}
	seq, sch.out = t.emitProjection(sch.columns, sch.isGrouped(), seq)
	if having != nil {
		seq = append(seq, sem.NewFilter(sel.Having, having))
	}
	if sel.Distinct {
		seq = t.genDistinct(sem.NewThis(sel, []string{"out"}), seq)
	}
	return seq, sch
}

// groupedExpr translates e in the context of any grouping expressions where
// we translate any matching expression to its aggregate tmp variable and
// flag an error if we encounter un-matched references to the input table.
// When the sel schema isn't an agg, this is like a normal t.expr().
func (t *translator) groupedExpr(sch *selectSchema, e ast.Expr) sem.Expr {
	if e != nil {
		if !sch.isGrouped() {
			return t.expr(e)
		}
		save := sch.grouped
		defer func() {
			sch.grouped = save
		}()
		sch.grouped = true
		if out, ok := replaceGroupings(t, t.expr(e), sch.groupings); ok {
			return out
		}
	}
	return nil
}

func (t *translator) replaceGroupings(sch *selectSchema, columns []column) {
	for k, c := range columns {
		columns[k].semExpr, _ = replaceGroupings(t, c.semExpr, sch.groupings)
	}
}

type column struct {
	name    string
	astExpr ast.Expr
	semExpr sem.Expr
	lateral bool
}

func (t *translator) formProjection(sch *selectSchema, in []ast.SQLAsExpr) []column {
	var out []column
	scores := make(map[string]int)
	for _, as := range in {
		if star, ok := as.Expr.(*ast.StarExpr); ok {
			// expand * and table.* expressions
			paths, err := sch.star(star, star.Table, nil)
			if err != nil {
				t.error(as, err)
			}
			for _, p := range paths {
				out = append(out, column{name: dedup(scores, t.asName(as.Label, nil, p.Path)), semExpr: p, astExpr: as.Expr})
			}
			continue
		}
		lateral := as.Label != nil && as.Label.Name != ""
		out = append(out, column{name: dedup(scores, t.asName(as.Label, as.Expr, nil)), astExpr: as.Expr, lateral: lateral})
	}
	return out
}

func (t *translator) asName(label *ast.ID, expr ast.Expr, path []string) string {
	var name string
	if label != nil {
		name = label.Name
		if name == "" {
			t.error(label, errors.New("column alias cannot be an empty string"))
		}
	} else if id, ok := expr.(*ast.IDExpr); ok && id.Name == "this" {
		name = "that"
	} else if path != nil {
		name = path[len(path)-1]
	} else {
		name = deriveNameFromExpr(expr)
	}
	return name
}

func (t *translator) projection(sch *selectSchema, columns []column) {
	for k := range columns {
		// Translates all expressions that weren't already expanded
		// from * patterns.
		sch.latStop = k
		if columns[k].semExpr == nil {
			columns[k].semExpr = t.expr(columns[k].astExpr)
		}
	}
	sch.latStop = len(sch.columns)
}

func (t *translator) emitProjection(columns []column, grouped bool, seq sem.Seq) (sem.Seq, *staticSchema) {
	if len(columns) == 0 {
		return seq, nil
	}
	var outs []sem.RecordElem
	var names []string
	for _, c := range columns {
		outs = append(outs, &sem.FieldElem{
			Node:  c.astExpr,
			Name:  c.name,
			Value: c.semExpr,
		})
		names = append(names, c.name)
	}
	var in string
	if grouped {
		in = "g"
	} else {
		in = "in"
	}
	loc := columns[0].astExpr //XXX
	e := &sem.RecordExpr{
		Node: loc,
		Elems: []sem.RecordElem{
			&sem.FieldElem{
				Node:  loc,
				Name:  in,
				Value: sem.NewThis(loc, field.Path{in}),
			},
			&sem.FieldElem{
				Node: loc,
				Name: "out",
				Value: &sem.RecordExpr{
					Node:  loc,
					Elems: outs,
				},
			},
		},
	}
	return append(seq, sem.NewValues(loc, e)), &staticSchema{columns: names}
}

func (t *translator) genAggregate(loc ast.Loc, sch *selectSchema, seq sem.Seq) sem.Seq {
	var aggCols []sem.Assignment
	for k, agg := range sch.aggs {
		a := sem.Assignment{
			Node: agg.Node,
			LHS:  sem.NewThis(agg.Node, []string{aggTmp(k)}),
			RHS:  agg,
		}
		aggCols = append(aggCols, a)
	}
	var keyCols []sem.Assignment
	for k, e := range sch.groupings {
		keyCols = append(keyCols, sem.Assignment{
			Node: e.loc,
			LHS:  sem.NewThis(e.loc, []string{groupTmp(k)}),
			RHS:  e.expr,
		})
	}
	return append(seq, &sem.AggregateOp{
		Node: loc,
		Aggs: aggCols,
		Keys: keyCols,
	})
}

func aggTmp(k int) string {
	return fmt.Sprintf("t%d", k)
}

func groupTmp(k int) string {
	return fmt.Sprintf("k%d", k)
}

func (t *translator) selectFrom(loc ast.Loc, exprs []ast.SQLTableExpr, seq sem.Seq) (sem.Seq, schema) {
	if len(exprs) == 0 {
		return seq, &dynamicSchema{}
	}
	off := len(seq)
	hasParent := off > 0
	seq, sch := t.sqlTableExpr(exprs[0], seq)
	if off >= len(seq) {
		// The chain didn't get lengthed so semFrom must have encountered
		// an error...
		return seq, badSchema
	}
	// If we have parents with both a from and select, report an error but
	// only if it's not a RobotScan where the parent feeds the from operateor.
	if _, ok := seq[off].(*sem.RobotScan); !ok {
		if hasParent {
			t.error(loc, errors.New("SELECT cannot have both an embedded FROM clause and input from parents"))
			return append(seq, badOp()), badSchema
		}
	}

	// Handle comma-separated table expressions in FROM clause.
	for _, e := range exprs[1:] {
		seq, sch = t.sqlAppendCrossJoin(e, seq, sch, e)
	}
	return seq, sch
}

func (t *translator) sqlValues(values *ast.SQLValues, seq sem.Seq) (sem.Seq, schema) {
	exprs := make([]sem.Expr, 0, len(values.Exprs))
	for _, astExpr := range values.Exprs {
		e := t.expr(astExpr)
		exprs = append(exprs, e)
	}
	seq = append(seq, sem.NewValues(values, exprs...))
	_, sch := t.inferSchema(exprs)
	return seq, sch
}

func (t *translator) inferSchema(exprs []sem.Expr) (super.Type, schema) {
	fuser := agg.NewSchema(t.sctx)
	for _, e := range exprs {
		val, ok := t.maybeEval(e)
		if !ok {
			return nil, &dynamicSchema{}
		}
		fuser.Mixin(val.Type())
	}
	recType, ok := super.TypeUnder(fuser.Type()).(*super.TypeRecord)
	if !ok {
		return nil, &dynamicSchema{}
	}
	columns := make([]string, 0, len(recType.Fields))
	for _, f := range recType.Fields {
		columns = append(columns, f.Name)
	}
	return recType, &staticSchema{columns: columns}
}

func (t *translator) genDistinct(e sem.Expr, seq sem.Seq) sem.Seq {
	return append(seq, &sem.DistinctOp{
		Node: e,
		Expr: e,
	})
}

func (t *translator) sqlPipe(pipe *ast.SQLPipe, seq sem.Seq) (sem.Seq, schema) {
	if query, ok := maybeSQLQueryBody(pipe); ok {
		return t.sqlQueryBody(query, nil, seq)
	}
	if len(seq) > 0 {
		panic("semSQLOp: SQL pipes can't have parents")
	}
	seq = t.seq(pipe.Body)
	// We pass in type null for initial type here since we know a pipe subquery inside
	// of a sql table expression must always have a data source (i.e., it cannot inherit
	// data from a parent node somewhere outside of this seq).  XXX this assumption may
	// change when we add support for correlated subqueries and an embedded pipe may
	// need access to the incoming type when feeding that type into the unnest scope
	// that implements the correlated subquery.
	return seq, newSchemaFromType(t.checker.seq(super.TypeNull, seq))
}

func maybeSQLQueryBody(pipe *ast.SQLPipe) (ast.SQLQueryBody, bool) {
	if len(pipe.Body) == 1 {
		if op, ok := pipe.Body[0].(*ast.SQLOp); ok {
			return op.Body, true
		}
	}
	return nil, false
}

func unfurl(n ast.Node, sch schema, seq sem.Seq) sem.Seq {
	if e := sch.unfurl(n); e != nil {
		return valuesExpr(e, seq)
	}
	return seq
}

func applyAlias(alias *ast.TableAlias, sch schema, seq sem.Seq) (sem.Seq, schema, error) {
	if alias == nil || sch == badSchema {
		return seq, sch, nil
	}
	if len(alias.Columns) == 0 {
		return seq, addTableAlias(sch, alias.Name), nil
	}
	if cols, ok := sch.outColumns(); ok {
		return mapColumns(cols, alias, seq)
	}
	return seq, sch, errors.New("cannot apply column aliases to dynamically typed data")
}

func mapColumns(in []string, alias *ast.TableAlias, seq sem.Seq) (sem.Seq, schema, error) {
	if len(alias.Columns) > len(in) {
		return nil, nil, fmt.Errorf("cannot apply %d column aliases in table alias %q to table with %d columns", len(alias.Columns), alias.Name, len(in))
	}
	out := idsToStrings(alias.Columns)
	if !slices.Equal(in, out) {
		// Make a record expression...
		elems := make([]sem.RecordElem, 0, len(in))
		for k := range out {
			elems = append(elems, &sem.FieldElem{
				Node:  alias.Columns[k],
				Name:  out[k],
				Value: sem.NewThis(alias.Columns[k], []string{in[k]}),
			})
		}
		seq = valuesExpr(&sem.RecordExpr{
			Node:  alias,
			Elems: elems,
		}, seq)
	}
	return seq, &staticSchema{table: alias.Name, columns: out}, nil
}

func idsToStrings(ids []*ast.ID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.Name)
	}
	return out
}

func (t *translator) sqlQueryBody(query ast.SQLQueryBody, demand []ast.Expr, seq sem.Seq) (sem.Seq, schema) {
	switch query := query.(type) {
	case *ast.SQLSelect:
		return t.sqlSelect(query, demand, seq)
	case *ast.SQLValues:
		return t.sqlValues(query, seq)
	case *ast.SQLQuery:
		if query.With != nil {
			old := t.sqlWith(query.With)
			defer func() { t.scope.ctes = old }()
		}
		var demand []ast.Expr
		if query.OrderBy != nil {
			demand = exprsFromSortExprs(query.OrderBy.Exprs)
		}
		seq, sch := t.sqlQueryBody(query.Body, demand, seq)
		if sch == badSchema {
			return seq, sch
		}
		if query.OrderBy != nil {
			seq = t.orderBy(query.OrderBy, sch, seq)
		}
		if limoff := query.Limit; limoff != nil {
			if limoff.Offset != nil {
				seq = append(seq, &sem.SkipOp{Node: limoff.Offset, Count: t.mustEvalPositiveInteger(limoff.Offset)})
			}
			if limoff.Limit != nil {
				seq = append(seq, &sem.HeadOp{Node: limoff.Limit, Count: t.mustEvalPositiveInteger(limoff.Limit)})
			}
		}
		return seq, sch
	case *ast.SQLUnion:
		left, leftSch := t.sqlQueryBody(query.Left, nil, seq)
		left, leftSch = leftSch.endScope(query.Left.(ast.Node), left)
		if leftSch == nil || leftSch == badSchema {
			// error happened in query body
			return sem.Seq{badOp()}, badSchema
		}
		leftCols, lok := leftSch.outColumns()
		if !lok {
			t.error(query.Left, errors.New("set operations cannot be applied to dynamic sources"))
		}
		right, rightSch := t.sqlQueryBody(query.Right, nil, seq)
		right, rightSch = rightSch.endScope(query.Right.(ast.Node), right)
		if rightSch == nil || rightSch == badSchema {
			// error happened in query body
			return sem.Seq{badOp()}, badSchema
		}
		rightCols, rok := rightSch.outColumns()
		if !rok {
			t.error(query.Right, errors.New("set operations cannot be applied to dynamic sources"))
		}
		if !lok || !rok {
			return sem.Seq{badOp()}, badSchema
		}
		if len(leftCols) != len(rightCols) {
			t.error(query, errors.New("set operations can only be applied to sources with the same number of columns"))
			return sem.Seq{badOp()}, badSchema
		}
		if !slices.Equal(leftCols, rightCols) {
			// Rename fields on the right to match the left.
			var elems []sem.RecordElem
			for i, col := range leftCols {
				elems = append(elems, &sem.FieldElem{
					Name: col,
					Value: &sem.IndexExpr{
						Expr:  sem.NewThis(nil, nil),
						Index: &sem.LiteralExpr{Value: strconv.Itoa(i)},
					},
				})
			}
			right = append(right, sem.NewValues(nil, &sem.RecordExpr{Elems: elems}))
		}
		out := sem.Seq{
			&sem.ForkOp{Node: query, Paths: []sem.Seq{left, right}},
			// This used to be dag.Combine but we don't have combine in the sem tree,
			// so we use a merge here.  If we don't put this in, then the optimizer
			// mysteriously removes the output/main from the end of the DAG.
			// The optimizer is too fussy/buggy in this way and we should clean it up.
			&sem.MergeOp{Node: query},
		}
		if query.Distinct {
			out = t.genDistinct(sem.NewThis(query, nil), out)
		}
		return out, leftSch
	default:
		panic(query)
	}
}

func exprsFromSortExprs(in []ast.SortExpr) []ast.Expr {
	var out []ast.Expr
	for _, e := range in {
		out = append(out, e.Expr)
	}
	return out
}

func (t *translator) orderBy(op *ast.SQLOrderBy, sch schema, seq sem.Seq) sem.Seq {
	save := t.scope.schema
	t.scope.schema = sch
	defer func() {
		t.scope.schema = save
	}()
	var exprs []sem.SortExpr
	for _, e := range op.Exprs {
		exprs = append(exprs, t.sortExpr(sch, e, false))
	}
	return append(seq, &sem.SortOp{Node: op, Exprs: exprs})
}

func (t *translator) sqlWith(with *ast.SQLWith) map[string]*ast.SQLCTE {
	if with.Recursive {
		t.error(with, errors.New("recursive WITH queries not currently supported"))
	}
	old := t.scope.ctes
	t.scope.ctes = maps.Clone(t.scope.ctes)
	for k, c := range with.CTEs {
		// XXX Materialized option not currently supported.
		name := strings.ToLower(c.Name.Name)
		if _, ok := t.scope.ctes[name]; ok {
			t.error(c.Name, errors.New("duplicate WITH clause name"))
		}
		t.scope.ctes[name] = &with.CTEs[k]
	}
	return old
}

func (t *translator) sqlCrossJoin(join *ast.SQLCrossJoin, seq sem.Seq) (sem.Seq, schema) {
	if len(seq) > 0 {
		// At some point we might want to let parent data flow into a join somehow,
		// but for now we flag an error.
		t.error(join, errors.New("SQL cross join cannot inherit data from pipeline parent"))
	}
	leftSeq, leftSchema := t.sqlTableExpr(join.Left, nil)
	return t.sqlAppendCrossJoin(join, leftSeq, leftSchema, join.Right)
}

func (t *translator) sqlAppendCrossJoin(node ast.Node, leftSeq sem.Seq, leftSchema schema, rhs ast.SQLTableExpr) (sem.Seq, schema) {
	rightSeq, rightSchema := t.sqlTableExpr(rhs, nil)
	sch := &joinSchema{left: leftSchema, right: rightSchema}
	par := &sem.ForkOp{Paths: []sem.Seq{leftSeq, rightSeq}}
	dagJoin := &sem.JoinOp{
		Node:       node,
		Style:      "cross",
		LeftAlias:  "left",
		RightAlias: "right",
	}
	return sem.Seq{par, dagJoin}, sch
}

// For now, each joining table is on the right...
// We don't have logic to not care about the side of the JOIN ON keys...
func (t *translator) sqlJoin(join *ast.SQLJoin, seq sem.Seq) (sem.Seq, schema) {
	if len(seq) > 0 {
		// At some point we might want to let parent data flow into a join somehow,
		// but for now we flag an error.
		t.error(join, errors.New("SQL join cannot inherit data from pipeline parent"))
	}
	leftSeq, leftSchema := t.sqlTableExpr(join.Left, nil)
	rightSeq, rightSchema := t.sqlTableExpr(join.Right, nil)
	var sch schema
	if using, ok := join.Cond.(*ast.JoinUsingCond); ok {
		sch = newJoinUsingSchema(&joinSchema{left: leftSchema, right: rightSchema}, idsToStrings(using.Fields))
	} else {
		sch = &joinSchema{left: leftSchema, right: rightSchema}
	}
	saved := t.scope.schema
	t.scope.schema = sch
	cond := t.sqlJoinCond(join.Cond)
	t.scope.schema = saved
	style := join.Style
	if style == "" {
		style = "inner"
	}
	par := &sem.ForkOp{Paths: []sem.Seq{leftSeq, rightSeq}}
	dagJoin := &sem.JoinOp{
		Node:       join,
		Style:      style,
		LeftAlias:  "left",
		RightAlias: "right",
		Cond:       cond,
	}
	return sem.Seq{par, dagJoin}, sch
}

func (t *translator) sqlJoinCond(cond ast.JoinCond) sem.Expr {
	switch cond := cond.(type) {
	case *ast.JoinOnCond:
		return t.expr(cond.Expr)
	case *ast.JoinUsingCond:
		jsch := t.scope.schema.(*joinUsingSchema).joinSchema
		var exprs []sem.Expr
		for _, id := range cond.Fields {
			left, _, err := jsch.left.resolveUnqualified(id.Name)
			if err != nil {
				t.error(id, err)
				continue
			}
			if left == nil {
				t.error(id, fmt.Errorf("column %q in USING clause does not exist in left table", id.Name))
				continue
			}
			right, _, err := jsch.right.resolveUnqualified(id.Name)
			if err != nil {
				t.error(id, err)
				continue
			}
			if right == nil {
				t.error(id, fmt.Errorf("column %q in USING clause does not exist in right table", id.Name))
				continue
			}
			lhs := sem.NewThis(id, append([]string{"left"}, left...))
			rhs := sem.NewThis(id, append([]string{"right"}, right...))
			exprs = append(exprs, sem.NewBinaryExpr(id, "==", lhs, rhs))
		}
		if len(exprs) == 0 {
			return badExpr()
		}
		return andUsingExprs(cond, exprs)
	default:
		panic(cond)
	}
}

func andUsingExprs(cond ast.Node, exprs []sem.Expr) sem.Expr {
	n := len(exprs)
	e := exprs[n-1]
	for i := n - 2; i >= 0; i-- {
		e = sem.NewBinaryExpr(cond, "and", exprs[i], e)
	}
	return e
}

func (t *translator) pipeJoinCond(cond ast.JoinCond, leftAlias, rightAlias string) sem.Expr {
	switch cond := cond.(type) {
	case *ast.JoinOnCond:
		e := t.expr(cond.Expr)
		// hack: e is wrapped in []sem.Expr to work around CanSet() model in WalkT
		dag.WalkT(reflect.ValueOf([]sem.Expr{e}), func(e *sem.ThisExpr) *sem.ThisExpr {
			if len(e.Path) == 0 {
				t.error(cond.Expr, errors.New(`join expression cannot refer to "this"`))
			} else if name := e.Path[0]; name != leftAlias && name != rightAlias {
				t.error(cond.Expr, fmt.Errorf("ambiguous field reference %q", name))
			}
			return e
		})
		return e
	case *ast.JoinUsingCond:
		var exprs []sem.Expr
		for _, id := range cond.Fields {
			lhs := sem.NewThis(id, []string{leftAlias, id.Name})
			rhs := sem.NewThis(id, []string{rightAlias, id.Name})
			exprs = append(exprs, sem.NewBinaryExpr(id, "==", lhs, rhs))
		}
		return andUsingExprs(cond, exprs)
	default:
		panic(cond)
	}
}

type exprloc struct {
	expr sem.Expr
	loc  ast.Expr
}

func (t *translator) groupBy(sch *selectSchema, in []ast.Expr) []exprloc {
	var out []exprloc
	for _, expr := range in {
		var e sem.Expr
		if colno, ok := isOrdinal(expr); ok {
			if colno < 1 || colno > len(sch.columns) {
				t.error(expr, fmt.Errorf("position %d is not in select list", colno))
			} else {
				save := sch.groupByLoc
				sch.groupByLoc = expr
				e = t.expr(sch.columns[colno-1].astExpr)
				sch.groupByLoc = save
			}
		} else {
			e = t.expr(expr)
		}
		out = append(out, exprloc{e, expr})
	}
	return out
}

func isOrdinal(e ast.Expr) (int, bool) {
	if e, ok := e.(*ast.Primitive); ok && e.Type == "int64" {
		colno, err := strconv.Atoi(e.Text)
		if err != nil {
			panic(err)
		}
		return colno, true
	}
	return -1, false
}

func (t *translator) resolveOrdinalOuter(s schema, n ast.Node, prefix string, col int) sem.Expr {
	switch s := s.(type) {
	case *selectSchema:
		return t.resolveOrdinalOuter(s.out, n, "out", col)
	case *staticSchema:
		if col < 1 || col > len(s.columns) {
			t.error(n, fmt.Errorf("column %d is out of range", col))
			return badExpr()
		}
		var path []string
		if prefix != "" {
			path = []string{prefix, s.columns[col-1]}
		} else {
			path = []string{s.columns[col-1]}
		}
		return sem.NewThis(n, path)
	default:
		panic(s)
	}
}

func dedup(scores map[string]int, s string) string {
	cnt := scores[s]
	scores[s] = cnt + 1
	if cnt != 0 {
		s = fmt.Sprintf("%s_%d", s, cnt)
	}
	return s
}

func valuesExpr(e sem.Expr, seq sem.Seq) sem.Seq {
	return append(seq, sem.NewValues(e, e))
}

func wrapThis(n ast.Node, field string) *sem.RecordExpr {
	return wrapField(field, sem.NewThis(n, nil))
}

func wrapField(field string, e sem.Expr) *sem.RecordExpr {
	return &sem.RecordExpr{
		Node: e,
		Elems: []sem.RecordElem{
			&sem.FieldElem{
				Node:  e,
				Name:  field,
				Value: e,
			},
		},
	}
}
