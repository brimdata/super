package semantic

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
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
func (t *translator) semSelect(sel *ast.SQLSelect, seq sem.Seq) (sem.Seq, schema) {
	if len(sel.Selection.Args) == 0 {
		t.error(sel, errors.New("SELECT clause has no selection"))
		return seq, badSchema()
	}
	seq, fromSchema := t.semSelectFrom(sel.Loc, sel.From, seq)
	if fromSchema == nil {
		return seq, badSchema()
	}
	if sel.Value {
		return t.semSelectValue(sel, fromSchema, seq)
	}
	if t.scope.schema != nil {
		fromSchema = &subquerySchema{
			outer: t.scope.schema,
			inner: fromSchema,
		}
	}
	sch := &selectSchema{in: fromSchema}
	var funcs aggfuncs
	proj := t.semProjection(sch, sel.Selection.Args, &funcs)
	var where sem.Expr
	if sel.Where != nil {
		where = t.semExprSchema(sch, sel.Where)
	}
	keyExprs := t.semGroupBy(sch, sel.GroupBy)
	having, err := t.semHaving(sch, sel.Having, &funcs)
	if err != nil {
		t.error(sel.Having, err)
		return seq, badSchema()
	}
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

func (t *translator) semHaving(sch schema, e ast.Expr, funcs *aggfuncs) (sem.Expr, error) {
	if e == nil {
		return nil, nil
	}
	return funcs.subst(t.semExprSchema(sch, e))
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
				Expr: sem.NewThis(col.expr, []string{"out"}),
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
	case *dynamicSchema:
		return append(elems, &sem.SpreadElem{
			Node: n,
			Expr: sem.NewThis(n, prefix),
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
		which := exprMatch(col.expr, keyExprs)
		if which < 0 {
			// Look for an exact-match of a column alias which would
			// convert to path out.<id> in the name resolution of the
			// grouping expression.
			alias := sem.NewThis(col.expr, []string{"out", col.name})
			which = exprMatch(alias, keyExprs)
		}
		if col.isAgg {
			if which >= 0 {
				t.error(keyExprs[which].loc, fmt.Errorf("aggregate functions are not allowed in GROUP BY"))
			}
			elems = append(elems, &sem.FieldElem{
				Node:  col.loc,
				Name:  col.name,
				Value: col.expr,
			})
		} else {
			if which < 0 {
				t.error(col.loc, fmt.Errorf("no corresponding grouping element for non-aggregate %q", col.name))
			}
			elems = append(elems, &sem.FieldElem{
				Node:  col.loc,
				Name:  col.name,
				Value: sem.NewThis(col.loc, []string{"in", fmt.Sprintf("k%d", which)}),
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

func (t *translator) semSelectFrom(loc ast.Loc, from *ast.From, seq sem.Seq) (sem.Seq, schema) {
	if from == nil {
		return seq, &dynamicSchema{}
	}
	off := len(seq)
	hasParent := off > 0
	seq, sch := t.semFrom(from, seq)
	if off >= len(seq) {
		// The chain didn't get lengthed so semFrom must have enocounteded
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
	return seq, sch
}

func (t *translator) semSelectValue(sel *ast.SQLSelect, sch schema, seq sem.Seq) (sem.Seq, schema) {
	if sel.GroupBy != nil {
		t.error(sel, errors.New("SELECT VALUE cannot be used with GROUP BY"))
		seq = append(seq, badOp())
	}
	if sel.Having != nil {
		t.error(sel, errors.New("SELECT VALUE cannot be used with HAVING"))
		seq = append(seq, badOp())
	}
	exprs := make([]sem.Expr, 0, len(sel.Selection.Args))
	for _, as := range sel.Selection.Args {
		if as.Label != nil {
			t.error(sel, errors.New("SELECT VALUE cannot have AS clause in selection"))
		}
		var e sem.Expr
		if as.Expr == nil {
			e = sem.NewThis(as, nil)
		} else {
			e = t.semExprSchema(sch, as.Expr)
		}
		exprs = append(exprs, e)
	}
	if sel.Where != nil {
		seq = append(seq, sem.NewFilter(sel.Where, t.semExprSchema(sch, sel.Where)))
	}
	seq = append(seq, sem.NewValues(sel, exprs...))
	if sel.Distinct {
		seq = t.genDistinct(sem.NewThis(sel, nil), seq)
	}
	return seq, &dynamicSchema{}
}

func (t *translator) semValues(values *ast.SQLValues, seq sem.Seq) (sem.Seq, schema) {
	exprs := make([]sem.Expr, 0, len(values.Exprs))
	for _, astExpr := range values.Exprs {
		e := t.semExpr(astExpr)
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

func (t *translator) semSQLPipe(op *ast.SQLPipe, seq sem.Seq, alias *ast.TableAlias) (sem.Seq, schema) {
	if len(op.Ops) == 1 && isSQLOp(op.Ops[0]) {
		seq, sch := t.semSQLOp(op.Ops[0], seq)
		outSeq, outSch, err := derefSchemaWithAlias(op.Ops[0], sch, alias, seq)
		if err != nil {
			t.error(op.Ops[0], err)
		}
		return outSeq, outSch
	}
	if len(seq) > 0 {
		panic("semSQLOp: SQL pipes can't have parents")
	}
	var name string
	if alias != nil {
		name = alias.Name
		if len(alias.Columns) != 0 {
			t.error(alias, errors.New("cannot apply column aliases to dynamically typed data"))
		}
	}
	return t.semSeq(op.Ops), &dynamicSchema{name: name}
}

func derefSchemaAs(n ast.Node, sch schema, table string, seq sem.Seq) (sem.Seq, schema) {
	e, sch := sch.deref(n, table)
	if e != nil {
		seq = valuesExpr(e, seq)
	}
	return seq, sch
}

func derefSchema(n ast.Node, sch schema, seq sem.Seq) (sem.Seq, schema) {
	return derefSchemaAs(n, sch, "", seq)
}

func derefSchemaWithAlias(n ast.Node, insch schema, alias *ast.TableAlias, inseq sem.Seq) (sem.Seq, schema, error) {
	var table string
	if alias != nil {
		table = alias.Name
	}
	seq, sch := derefSchemaAs(n, insch, table, inseq)
	if alias == nil || len(alias.Columns) == 0 {
		return seq, sch, nil
	}
	if sch, ok := sch.(*staticSchema); ok {
		return mapColumns(sch.columns, alias, seq)
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
	return seq, &staticSchema{alias.Name, out}, nil
}

func idsToStrings(ids []*ast.ID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.Name)
	}
	return out
}

func isSQLOp(op ast.Op) bool {
	switch op.(type) {
	case *ast.SQLSelect, *ast.SQLLimitOffset, *ast.SQLOrderBy, *ast.SQLPipe, *ast.SQLJoin, *ast.SQLValues:
		return true
	}
	return false
}

func (t *translator) semSQLOp(op ast.Op, seq sem.Seq) (sem.Seq, schema) {
	switch op := op.(type) {
	case *ast.SQLPipe:
		return t.semSQLPipe(op, seq, nil) //XXX should alias hang off SQLPipe?
	case *ast.SQLSelect:
		return t.semSelect(op, seq)
	case *ast.SQLValues:
		return t.semValues(op, seq)
	case *ast.SQLJoin:
		return t.semSQLJoin(op, seq)
	case *ast.SQLOrderBy:
		out, schema := t.semSQLOp(op.Op, seq)
		var exprs []sem.SortExpr
		for _, e := range op.Exprs {
			exprs = append(exprs, t.semSortExpr(schema, e, false))
		}
		return append(out, &sem.SortOp{Node: op, Exprs: exprs}), schema
	case *ast.SQLLimitOffset:
		out, schema := t.semSQLOp(op.Op, seq)
		if op.Offset != nil {
			out = append(out, &sem.SkipOp{Node: op.Offset, Count: t.mustEvalPositiveInteger(op.Offset)})
		}
		if op.Limit != nil {
			out = append(out, &sem.HeadOp{Node: op.Limit, Count: t.mustEvalPositiveInteger(op.Limit)})
		}
		return out, schema
	case *ast.SQLUnion:
		left, leftSch := t.semSQLOp(op.Left, seq)
		left, _ = derefSchema(op.Left, leftSch, left)
		right, rightSch := t.semSQLOp(op.Right, seq)
		right, _ = derefSchema(op.Right, rightSch, right)
		out := sem.Seq{
			&sem.ForkOp{Node: op, Paths: []sem.Seq{left, right}},
			// This used to be dag.Combine but we don't have combine in the sem tree,
			// so we use a merge here.  If we don't put this in, then the optimizer
			// mysteriously removes the output/main from the end of the DAG.
			// The optimizer is too fussy/buggy in this way and we should clean it up.
			&sem.MergeOp{Node: op},
		}
		if op.Distinct {
			out = t.genDistinct(sem.NewThis(op, nil), out)
		}
		return out, &dynamicSchema{}

	case *ast.SQLWith:
		if op.Recursive {
			t.error(op, errors.New("recursive WITH queries not currently supported"))
		}
		old := t.scope.ctes
		t.scope.ctes = maps.Clone(t.scope.ctes)
		defer func() { t.scope.ctes = old }()
		for k, c := range op.CTEs {
			// XXX Materialized option not currently supported.
			name := strings.ToLower(c.Name.Name)
			if _, ok := t.scope.ctes[name]; ok {
				t.error(c.Name, errors.New("duplicate WITH clause name"))
			}
			t.scope.ctes[name] = &op.CTEs[k]
		}
		return t.semSQLOp(op.Body, seq)
	default:
		panic(fmt.Sprintf("semSQLOp: unknown op: %#v", op))
	}
}

func (t *translator) semCrossJoin(join *ast.SQLCrossJoin, seq sem.Seq) (sem.Seq, schema) {
	if len(seq) > 0 {
		// At some point we might want to let parent data flow into a join somehow,
		// but for now we flag an error.
		t.error(join, errors.New("SQL cross join cannot inherit data from pipeline parent"))
	}
	leftSeq, leftSchema := t.semFromElem(join.Left, nil)
	rightSeq, rightSchema := t.semFromElem(join.Right, nil)
	sch := &joinSchema{left: leftSchema, right: rightSchema}
	par := &sem.ForkOp{Paths: []sem.Seq{leftSeq, rightSeq}}
	dagJoin := &sem.JoinOp{
		Node:       join,
		Style:      "cross",
		LeftAlias:  "left",
		RightAlias: "right",
	}
	return sem.Seq{par, dagJoin}, sch
}

// For now, each joining table is on the right...
// We don't have logic to not care about the side of the JOIN ON keys...
func (t *translator) semSQLJoin(join *ast.SQLJoin, seq sem.Seq) (sem.Seq, schema) {
	if len(seq) > 0 {
		// At some point we might want to let parent data flow into a join somehow,
		// but for now we flag an error.
		t.error(join, errors.New("SQL join cannot inherit data from pipeline parent"))
	}
	leftSeq, leftSchema := t.semFromElem(join.Left, nil)
	rightSeq, rightSchema := t.semFromElem(join.Right, nil)
	sch := &joinSchema{left: leftSchema, right: rightSchema}
	saved := t.scope.schema
	t.scope.schema = sch
	cond := t.semJoinCond(join.Cond, "left", "right")
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

func (t *translator) semJoinCond(cond ast.JoinCond, leftAlias, rightAlias string) sem.Expr {
	switch cond := cond.(type) {
	case *ast.JoinOnCond:
		if id, ok := cond.Expr.(*ast.ID); ok {
			return t.semJoinCond(&ast.JoinUsingCond{Fields: []ast.Expr{id}}, leftAlias, rightAlias)
		}
		e := t.semExpr(cond.Expr)
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
		for _, e := range t.semFields(cond.Fields) {
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

func (t *translator) semGroupBy(sch *selectSchema, in []ast.Expr) []exprloc {
	var out []exprloc
	var funcs aggfuncs
	for k, expr := range in {
		e := t.semExprSchema(sch, expr)
		if which, ok := isOrdinal(t.sctx, e); ok {
			var err error
			if e, err = sch.resolveOrdinal(e, which); err != nil {
				t.error(in[k], err)
			}
		}
		// Grouping expressions can't have agg funcs so we parse as a column
		// and see if any agg functions were found.
		c, _ := newColumn("", in[k], e, &funcs)
		if c != nil && c.isAgg {
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

func (t *translator) semProjection(sch *selectSchema, args []ast.SQLAsExpr, funcs *aggfuncs) projection {
	out := &staticSchema{}
	sch.out = out
	labels := make(map[string]struct{})
	var proj projection
	for _, as := range args {
		if isStar(as) {
			proj = append(proj, column{})
			continue
		}
		col := t.semAs(sch, as, funcs)
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
			if static, ok := sch.in.(*staticSchema); ok {
				out.columns = append(out.columns, static.columns...)
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

func (t *translator) semAs(sch schema, as ast.SQLAsExpr, funcs *aggfuncs) *column {
	e := t.semExprSchema(sch, as.Expr)
	// If we have a name from an AS clause, use it. Otherwise, infer a name.
	var name string
	if as.Label != nil {
		name = as.Label.Name
		if name == "" {
			t.error(as.Label, errors.New("label cannot be an empty string"))
		}
	} else {
		name = deriveNameFromExpr(e, as.Expr)
	}
	c, err := newColumn(name, as.Expr, e, funcs)
	if err != nil {
		t.error(as, err)
	}
	return c
}

func (t *translator) semExprSchema(s schema, e ast.Expr) sem.Expr {
	save := t.scope.schema
	t.scope.schema = s
	out := t.semExpr(e)
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
