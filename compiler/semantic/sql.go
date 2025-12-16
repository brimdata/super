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
	"github.com/brimdata/super/sup"
)

// Analyze a SQL select expression which may have arbitrary nested subqueries
// and may or may not have its sources embedded.
// The output of a select expression is a record that wraps its input and
// selected columns in a record {in:any,out:any}.  The schema returned represents
// the observable scope of the selected elements.  When the parent operator is
// an OrderBy, it can reach into the "in" part of the select scope (for non-aggregates)
// and also sort by the out elements.  It's up to the caller to unwrap the in/out
// record when returning to pipeline context.
func (t *translator) sqlSelect(sel *ast.SQLSelect, seq sem.Seq) (sem.Seq, schema) {
	if len(sel.Selection.Args) == 0 {
		t.error(sel, errors.New("SELECT clause has no selection"))
		return seq, badSchema()
	}
	seq, fromSchema := t.selectFrom(sel.Loc, sel.From, seq)
	if fromSchema == nil {
		return seq, badSchema()
	}
	if t.scope.schema != nil {
		fromSchema = &subquerySchema{
			outer: t.scope.schema,
			inner: fromSchema,
		}
	}
	sch := &selectSchema{in: fromSchema}
	var funcs aggfuncs
	proj := t.projection(sch, sel.Selection.Args, &funcs)
	var where sem.Expr
	if sel.Where != nil {
		where = t.exprSchema(sch, sel.Where)
	}
	keyExprs := t.groupBy(sch, sel.GroupBy)
	having := t.having(sch, sel.Having, &funcs)
	// Now that all the pieces have been converted to sem tree fragments,
	// we stitch together the fragments into pipeline operators depending
	// on whether its an aggregation or a selection of scalar expressions.
	seq = valuesExpr(wrapThis(sel, "in"), seq)
	if len(funcs) != 0 || len(keyExprs) != 0 {
		seq = t.genAggregate(sel.Loc, proj, where, keyExprs, funcs, having, seq)
		return seq, sch.out
	}
	if having != nil {
		t.error(sel.Having, errors.New("HAVING clause requires aggregations and/or a GROUP BY clause"))
	}
	seq = t.genValues(proj, where, sch, seq)
	if sel.Distinct {
		seq = t.genDistinct(sem.NewThis(sel, []string{"out"}), seq)
	}
	return seq, sch
}

func (t *translator) having(sch *selectSchema, e ast.Expr, funcs *aggfuncs) sem.Expr {
	if e == nil {
		return nil
	}
	return funcs.subst(t.exprSchema(&havingSchema{sch}, e))
}

func (t *translator) genValues(proj projection, where sem.Expr, sch *selectSchema, seq sem.Seq) sem.Seq {
	if len(proj) == 0 {
		return nil
	}
	seq = t.genColumns(proj, sch, seq)
	if where != nil {
		seq = append(seq, sem.NewFilter(where, where))
	}
	return seq
}

func (t *translator) genColumns(proj projection, sch *selectSchema, seq sem.Seq) sem.Seq {
	var notFirst bool
	for _, col := range proj {
		if col.isAgg {
			continue
		}
		var elems []sem.RecordElem
		if notFirst {
			elems = append(elems, &sem.SpreadElem{
				Node: col.loc,
				Expr: sem.NewThis(col.loc, []string{"out"}),
			})
		} else {
			notFirst = true
		}
		if col.isStar() {
			elems = unravel(col.loc, elems, sch, nil)
		} else {
			elems = append(elems, &sem.FieldElem{
				Node:  col.loc,
				Name:  col.name,
				Value: col.expr,
			})
		}
		e := &sem.RecordExpr{
			Node: col.loc,
			Elems: []sem.RecordElem{
				&sem.FieldElem{
					Node:  col.loc,
					Name:  "in",
					Value: sem.NewThis(col.loc, field.Path{"in"}),
				},
				&sem.FieldElem{
					Node: col.loc,
					Name: "out",
					Value: &sem.RecordExpr{
						Node:  col.loc,
						Elems: elems,
					},
				},
			},
		}
		seq = append(seq, sem.NewValues(col.loc, e))
	}
	return seq
}

func unravel(n ast.Node, elems []sem.RecordElem, schema schema, prefix field.Path) []sem.RecordElem {
	switch schema := schema.(type) {
	case *aliasSchema:
		return unravel(n, elems, schema.sch, prefix)
	case *dynamicSchema:
		return append(elems, &sem.SpreadElem{
			Node: n,
			Expr: sem.NewThis(n, slices.Clone(prefix)),
		})
	case *staticSchema:
		for _, col := range schema.columns {
			elems = append(elems, &sem.FieldElem{
				Name:  col,
				Value: sem.NewThis(n, slices.Clone(append(prefix, col))),
			})
		}
		return elems
	case *selectSchema:
		return unravel(n, elems, schema.in, append(prefix, "in"))
	case *joinSchema:
		elems = unravel(n, elems, schema.left, append(prefix, "left"))
		return unravel(n, elems, schema.right, append(prefix, "right"))
	case *subquerySchema:
		// XXX we're currently using subquerySchema to detect correlated subqueries
		// but not doing the unnest yet, so in this case here (e.g., (select * from...)),
		// we just unravel the inner schema without a path extension. This will change
		// when we implement proper SQL subqueries with correlated subquery support.
		return unravel(n, elems, schema.inner, prefix)
	default:
		panic(schema)
	}
}

func (t *translator) genAggregate(loc ast.Loc, proj projection, where sem.Expr, keyExprs []exprloc, funcs aggfuncs, having sem.Expr, seq sem.Seq) sem.Seq {
	if proj.hasStar() {
		// XXX take this out and figure out to incorporate this especially if we know the input schema
		t.error(loc, errors.New("aggregate mixed with *-selector not yet supported"))
		return append(seq, badOp())
	}
	if len(proj) != len(proj.aggCols()) {
		// Yield expressions for potentially left-to-right-dependent
		// column expressions of the grouping expression components.
		seq = t.genValues(proj, nil, nil, seq)
	}
	if where != nil {
		seq = append(seq, sem.NewFilter(loc, where))
	}
	var aggCols []sem.Assignment
	for _, named := range funcs {
		a := sem.Assignment{
			Node: named.agg.Node,
			LHS:  sem.NewThis(named.agg.Node, []string{named.name}),
			RHS:  named.agg,
		}
		aggCols = append(aggCols, a)
	}
	var keyCols []sem.Assignment
	for k, e := range keyExprs {
		keyCols = append(keyCols, sem.Assignment{
			Node: e.loc,
			LHS:  sem.NewThis(e.loc, []string{fmt.Sprintf("k%d", k)}),
			RHS:  e.expr,
		})
	}
	seq = append(seq, &sem.AggregateOp{
		Node: loc,
		Aggs: aggCols,
		Keys: keyCols,
	})
	seq = valuesExpr(wrapThis(loc, "in"), seq)
	seq = t.genAggregateOutput(loc, proj, keyExprs, seq)
	if having != nil {
		seq = append(seq, sem.NewFilter(having, having))
	}
	return valuesExpr(sem.NewThis(loc, []string{"out"}), seq)
}

func (t *translator) genAggregateOutput(loc ast.Node, proj projection, keyExprs []exprloc, seq sem.Seq) sem.Seq {
	notFirst := false
	for _, col := range proj {
		if col.isStar() {
			// XXX this turns into grouping keys (this)
			panic("TBD")
		}
		var elems []sem.RecordElem
		if notFirst {
			elems = append(elems, &sem.SpreadElem{
				Node: col.loc,
				Expr: sem.NewThis(col.loc, []string{"out"}),
			})
		} else {
			notFirst = true
		}
		// First, try to match the column expression to one of the grouping
		// expressions.  If that doesn't work, see if the aliased column
		// name is one of the grouping expressions.
		key, ok := keySubst(sem.CopyExpr(col.expr), keyExprs)
		if !ok {
			alias := sem.NewThis(col.loc, []string{"out", col.name})
			if which := exprMatch(alias, keyExprs); which >= 0 {
				key = sem.NewThis(col.loc, []string{"in", fmt.Sprintf("k%d", which)})
				ok = true
			}
		}
		if col.isAgg {
			if ok {
				t.error(col.expr, fmt.Errorf("aggregate functions are not allowed in GROUP BY"))
			}
			elems = append(elems, &sem.FieldElem{
				Node:  col.loc,
				Name:  col.name,
				Value: col.expr,
			})
		} else {
			if !ok {
				t.error(col.loc, fmt.Errorf("no corresponding grouping element for non-aggregate %q", col.name))
			}
			elems = append(elems, &sem.FieldElem{
				Node:  col.loc,
				Name:  col.name,
				Value: key,
			})
		}
		e := &sem.RecordExpr{
			Node: loc,
			Elems: []sem.RecordElem{
				&sem.FieldElem{
					Node:  loc,
					Name:  "in",
					Value: sem.NewThis(loc, field.Path{"in"}),
				},
				&sem.FieldElem{
					Node: loc,
					Name: "out",
					Value: &sem.RecordExpr{
						Node:  loc,
						Elems: elems,
					},
				},
			},
		}
		seq = append(seq, sem.NewValues(loc, e))
	}
	return seq
}

func exprMatch(target sem.Expr, exprs []exprloc) int {
	for which, e := range exprs {
		if eqExpr(target, e.expr) {
			return which
		}
	}
	return -1
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
		return seq, nil
	}
	// If we have parents with both a from and select, report an error but
	// only if it's not a RobotScan where the parent feeds the from operateor.
	if _, ok := seq[off].(*sem.RobotScan); !ok {
		if hasParent {
			t.error(loc, errors.New("SELECT cannot have both an embedded FROM clause and input from parents"))
			return append(seq, badOp()), nil
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
		return t.sqlQueryBody(query, seq)
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

func endScope(n ast.Node, sch schema, seq sem.Seq) (sem.Seq, schema) {
	e, sch := sch.endScope(n)
	if e != nil {
		seq = valuesExpr(e, seq)
	}
	return seq, sch
}

func unfurl(n ast.Node, sch schema, seq sem.Seq) sem.Seq {
	if e := sch.unfurl(n); e != nil {
		return valuesExpr(e, seq)
	}
	return seq
}

func applyAlias(alias *ast.TableAlias, sch schema, seq sem.Seq) (sem.Seq, schema, error) {
	if alias == nil {
		return seq, sch, nil
	}
	if len(alias.Columns) == 0 {
		return seq, addAlias(sch, alias.Name), nil
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
	return seq, &aliasSchema{alias.Name, &staticSchema{out}}, nil
}

func idsToStrings(ids []*ast.ID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.Name)
	}
	return out
}

func (t *translator) sqlQueryBody(query ast.SQLQueryBody, seq sem.Seq) (sem.Seq, schema) {
	switch query := query.(type) {
	case *ast.SQLSelect:
		return t.sqlSelect(query, seq)
	case *ast.SQLValues:
		return t.sqlValues(query, seq)
	case *ast.SQLQuery:
		if query.With != nil {
			old := t.sqlWith(query.With)
			defer func() { t.scope.ctes = old }()
		}
		seq, sch := t.sqlQueryBody(query.Body, seq)
		if query.OrderBy != nil {
			var exprs []sem.SortExpr
			for _, e := range query.OrderBy.Exprs {
				exprs = append(exprs, t.sortExpr(sch, e, false))
			}
			seq = append(seq, &sem.SortOp{Node: query.OrderBy, Exprs: exprs})
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
		left, leftSch := t.sqlQueryBody(query.Left, seq)
		left, leftSch = endScope(query.Left.(ast.Node), leftSch, left)
		leftCols, lok := leftSch.outColumns()
		if !lok {
			t.error(query.Left, errors.New("set operations cannot be applied to dynamic sources"))
		}
		right, rightSch := t.sqlQueryBody(query.Right, seq)
		right, rightSch = endScope(query.Right.(ast.Node), rightSch, right)
		rightCols, rok := rightSch.outColumns()
		if !rok {
			t.error(query.Right, errors.New("set operations cannot be applied to dynamic sources"))
		}
		if !lok || !rok {
			return sem.Seq{badOp()}, badSchema()
		}
		if len(leftCols) != len(rightCols) {
			t.error(query, errors.New("set operations can only be applied to sources with the same number of columns"))
			return sem.Seq{badOp()}, badSchema()
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
	sch := &joinSchema{left: leftSchema, right: rightSchema}
	saved := t.scope.schema
	t.scope.schema = sch
	cond := t.joinCond(join.Cond, "left", "right")
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

func (t *translator) joinCond(cond ast.JoinCond, leftAlias, rightAlias string) sem.Expr {
	switch cond := cond.(type) {
	case *ast.JoinOnCond:
		if id, ok := cond.Expr.(*ast.IDExpr); ok {
			return t.joinCond(&ast.JoinUsingCond{Fields: []ast.Expr{id}}, leftAlias, rightAlias)
		}
		e := t.expr(cond.Expr)
		dag.WalkT(reflect.ValueOf(e), func(e *sem.ThisExpr) *sem.ThisExpr {
			if len(e.Path) == 0 {
				t.error(cond.Expr, errors.New(`join expression cannot refer to "this"`))
			} else if name := e.Path[0]; name != leftAlias && name != rightAlias {
				t.error(cond.Expr, fmt.Errorf("ambiguous field reference %q", name))
			}
			return e
		})
		return e
	case *ast.JoinUsingCond:
		if t.scope.schema != nil {
			sch := t.scope.schema.(*joinSchema)
			t.scope.schema = &joinUsingSchema{sch}
			defer func() { t.scope.schema = sch }()
		}
		var exprs []sem.Expr
		for _, e := range t.fields(cond.Fields) {
			switch ee := e.(type) {
			case *sem.BadExpr:
			case *sem.ThisExpr:
				lhs := sem.NewThis(ee, append([]string{leftAlias}, ee.Path...))
				rhs := sem.NewThis(ee, append([]string{rightAlias}, ee.Path...))
				e = sem.NewBinaryExpr(ee, "==", lhs, rhs)
			default:
				panic(ee)
			}
			exprs = append(exprs, e)
		}
		n := len(exprs)
		e := exprs[n-1]
		for i := n - 2; i >= 0; i-- {
			e = sem.NewBinaryExpr(cond, "and", exprs[i], e)
		}
		return e
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
	var funcs aggfuncs
	for k, expr := range in {
		e := t.exprSchema(sch, expr)
		if which, ok := isOrdinal(t.sctx, e); ok {
			var err error
			if e, err = sch.resolveOrdinal(e, which); err != nil {
				t.error(in[k], err)
			}
		}
		// Grouping expressions can't have agg funcs so we parse as a column
		// and see if any agg functions were found.
		if newColumn("", in[k], e, &funcs).isAgg {
			t.error(in[k], errors.New("aggregate function cannot appear in GROUP BY clause"))
		}
		out = append(out, exprloc{e, expr})
	}
	return out
}

func isOrdinal(sctx *super.Context, e sem.Expr) (int, bool) {
	if literal, ok := e.(*sem.LiteralExpr); ok {
		v := sup.MustParseValue(sctx, literal.Value)
		return int(v.AsInt()), super.IsInteger(v.Type().ID())
	}
	return -1, false
}

func (t *translator) projection(sch *selectSchema, args []ast.SQLAsExpr, funcs *aggfuncs) projection {
	out := &staticSchema{}
	sch.out = out
	labels := make(map[string]struct{})
	var proj projection
	for _, as := range args {
		if isStar(as) {
			proj = append(proj, column{loc: as})
			continue
		}
		col := t.as(sch, as, funcs)
		if as.Label != nil {
			if _, ok := labels[col.name]; ok {
				t.error(as.Label, fmt.Errorf("duplicate column label %q", col.name))
				continue
			}
			labels[col.name] = struct{}{}
		}
		proj = append(proj, *col)
	}
	// Do we have duplicates for inferred columns? If an alias and an
	// inferred column collide the inferred name should be changed.
	for i := range proj {
		col := &proj[i]
		if col.isStar() {
			if cols, ok := sch.in.outColumns(); ok {
				out.columns = append(out.columns, cols...)
			} else {
				sch.out = &dynamicSchema{}
			}
			continue
		}
		if args[i].Label == nil {
			col.name = dedupeColname(labels, col.name)
		}
		labels[col.name] = struct{}{}
		out.columns = append(out.columns, col.name)
	}
	return proj
}

func dedupeColname(m map[string]struct{}, name string) string {
	for i := 0; ; i++ {
		s := name
		if i > 0 {
			s += fmt.Sprintf("_%d", i)
		}
		if _, ok := m[s]; !ok {
			return s
		}
	}
}

func isStar(a ast.SQLAsExpr) bool {
	return a.Expr == nil && a.Label == nil
}

func (t *translator) as(sch schema, as ast.SQLAsExpr, funcs *aggfuncs) *column {
	e := t.exprSchema(sch, as.Expr)
	// If we have a name from an AS clause, use it. Otherwise, infer a name.
	var name string
	if as.Label != nil {
		name = as.Label.Name
		if name == "" {
			t.error(as.Label, errors.New("label cannot be an empty string"))
		}
	} else if id, ok := as.Expr.(*ast.IDExpr); ok && id.Name == "this" {
		name = "that"
	} else {
		name = deriveNameFromExpr(as.Expr)
	}
	return newColumn(name, as.Expr, e, funcs)
}

func (t *translator) exprSchema(s schema, e ast.Expr) sem.Expr {
	save := t.scope.schema
	t.scope.schema = s
	out := t.expr(e)
	t.scope.schema = save
	return out
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
