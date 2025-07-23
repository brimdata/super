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
	"github.com/brimdata/super/compiler/rungen"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zfmt"
)

// Analyze a SQL select expression which may have arbitrary nested subqueries
// and may or may not have its sources embedded.
// The output of a select expression is a record that wraps its input and
// selected columns in a record {in:any,out:any}.  The schema returned represents
// the observable scope of the selected elements.  When the parent operator is
// an OrderBy, it can reach into the "in" part of the select scope (for non-aggregates)
// and also sort by the out elements.  It's up to the caller to unwrap the in/out
// record when returning to pipeline context.
func (a *analyzer) semSelect(sel *ast.Select, seq dag.Seq) (dag.Seq, schema) {
	if len(sel.Selection.Args) == 0 {
		a.error(sel, errors.New("SELECT clause has no selection"))
		return seq, badSchema()
	}
	seq, fromSchema := a.semSelectFrom(sel.Loc, sel.From, seq)
	if fromSchema == nil {
		return seq, badSchema()
	}
	if sel.Value {
		return a.semSelectValue(sel, fromSchema, seq)
	}
	sch := &selectSchema{in: fromSchema}
	var funcs aggfuncs
	proj := a.semProjection(sch, sel.Selection.Args, &funcs)
	var where dag.Expr
	if sel.Where != nil {
		where = a.semExprSchema(sch, sel.Where)
	}
	keyExprs := a.semGroupBy(sch, sel.GroupBy)
	having, err := a.semHaving(sch, sel.Having, &funcs)
	if err != nil {
		a.error(sel.Having, err)
		return seq, badSchema()
	}
	// Now that all the pieces have been converted to DAG fragments,
	// we stitch together the fragments into pipeline operators depending
	// on whether its an aggregation or a selection of scalar expressions.
	seq = valuesExpr(wrapThis("in"), seq)
	if len(funcs) != 0 || len(keyExprs) != 0 {
		seq = a.genAggregate(sel.Loc, proj, where, keyExprs, funcs, having, seq)
		return seq, sch.out
	}
	if having != nil {
		a.error(sel.Having, errors.New("HAVING clause requires aggregations and/or a GROUP BY clause"))
	}
	seq = a.genValues(proj, where, sch, seq)
	if sel.Distinct {
		seq = a.genDistinct(pathOf("out"), seq)
	}
	return seq, sch
}

func (a *analyzer) semHaving(sch schema, e ast.Expr, funcs *aggfuncs) (dag.Expr, error) {
	if e == nil {
		return nil, nil
	}
	return funcs.subst(a.semExprSchema(sch, e))
}

func (a *analyzer) genValues(proj projection, where dag.Expr, sch *selectSchema, seq dag.Seq) dag.Seq {
	if len(proj) == 0 {
		return nil
	}
	seq = a.genColumns(proj, sch, seq)
	if where != nil {
		seq = append(seq, dag.NewFilter(where))
	}
	return seq
}

func (a *analyzer) genColumns(proj projection, sch *selectSchema, seq dag.Seq) dag.Seq {
	var notFirst bool
	for _, col := range proj {
		if col.isAgg {
			continue
		}
		var elems []dag.RecordElem
		if notFirst {
			elems = append(elems, &dag.Spread{
				Kind: "Spread",
				Expr: &dag.This{Kind: "This", Path: []string{"out"}},
			})
		} else {
			notFirst = true
		}
		if col.isStar() {
			for _, path := range unravel(sch, nil) {
				elems = append(elems, &dag.Spread{
					Kind: "Spread",
					Expr: &dag.This{Kind: "This", Path: path},
				})
			}
		} else {
			elems = append(elems, &dag.Field{
				Kind:  "Field",
				Name:  col.name,
				Value: col.expr,
			})
		}
		e := &dag.RecordExpr{
			Kind: "RecordExpr",
			Elems: []dag.RecordElem{
				&dag.Field{
					Kind:  "Field",
					Name:  "in",
					Value: &dag.This{Kind: "This", Path: field.Path{"in"}},
				},
				&dag.Field{
					Kind: "Field",
					Name: "out",
					Value: &dag.RecordExpr{
						Kind:  "RecordExpr",
						Elems: elems,
					},
				},
			},
		}
		seq = append(seq, dag.NewValues(e))
	}
	return seq
}

func unravel(schema schema, prefix field.Path) []field.Path {
	switch schema := schema.(type) {
	default:
		return []field.Path{prefix}
	case *selectSchema:
		return unravel(schema.in, append(prefix, "in"))
	case *joinSchema:
		out := unravel(schema.left, append(prefix, "left"))
		return append(out, unravel(schema.right, append(prefix, "right"))...)
	}
}

func (a *analyzer) genAggregate(loc ast.Loc, proj projection, where dag.Expr, keyExprs []exprloc, funcs aggfuncs, having dag.Expr, seq dag.Seq) dag.Seq {
	if proj.hasStar() {
		// XXX take this out and figure out to incorporate this especially if we know the input schema
		a.error(loc, errors.New("aggregate mixed with *-selector not yet supported"))
		return append(seq, badOp())
	}
	if len(proj) != len(proj.aggCols()) {
		// Yield expressions for potentially left-to-right-dependent
		// column expressions of the grouping expression components.
		seq = a.genValues(proj, nil, nil, seq)
	}
	if where != nil {
		seq = append(seq, dag.NewFilter(where))
	}
	var aggCols []dag.Assignment
	for _, named := range funcs {
		a := dag.Assignment{
			Kind: "Assignment",
			LHS:  &dag.This{Kind: "This", Path: []string{named.name}},
			RHS:  named.agg,
		}
		aggCols = append(aggCols, a)
	}
	var keyCols []dag.Assignment
	for k, e := range keyExprs {
		keyCols = append(keyCols, dag.Assignment{
			Kind: "Assignment",
			LHS:  &dag.This{Kind: "This", Path: []string{fmt.Sprintf("k%d", k)}},
			RHS:  e.expr,
		})
	}
	seq = append(seq, &dag.Aggregate{
		Kind: "Aggregate",
		Aggs: aggCols,
		Keys: keyCols,
	})
	seq = valuesExpr(wrapThis("in"), seq)
	seq = a.genAggregateOutput(proj, keyExprs, seq)
	if having != nil {
		seq = append(seq, dag.NewFilter(having))
	}
	return valuesExpr(pathOf("out"), seq)
}

func (a *analyzer) genAggregateOutput(proj projection, keyExprs []exprloc, seq dag.Seq) dag.Seq {
	notFirst := false
	for _, col := range proj {
		if col.isStar() {
			// XXX this turns into grouping keys (this)
			panic("TBD")
		}
		var elems []dag.RecordElem
		if notFirst {
			elems = append(elems, &dag.Spread{
				Kind: "Spread",
				Expr: &dag.This{Kind: "This", Path: []string{"out"}},
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
			alias := &dag.This{Kind: "This", Path: []string{"out", col.name}}
			which = exprMatch(alias, keyExprs)
		}
		if col.isAgg {
			if which >= 0 {
				a.error(keyExprs[which].loc, fmt.Errorf("aggregate functions are not allowed in GROUP BY"))
			}
			elems = append(elems, &dag.Field{
				Kind:  "Field",
				Name:  col.name,
				Value: col.expr,
			})
		} else {
			if which < 0 {
				a.error(col.loc, fmt.Errorf("no corresponding grouping element for non-aggregate %q", col.name))
			}
			elems = append(elems, &dag.Field{
				Kind:  "Field",
				Name:  col.name,
				Value: &dag.This{Kind: "This", Path: []string{"in", fmt.Sprintf("k%d", which)}},
			})
		}
		e := &dag.RecordExpr{
			Kind: "RecordExpr",
			Elems: []dag.RecordElem{
				&dag.Field{
					Kind:  "Field",
					Name:  "in",
					Value: &dag.This{Kind: "This", Path: field.Path{"in"}},
				},
				&dag.Field{
					Kind: "Field",
					Name: "out",
					Value: &dag.RecordExpr{
						Kind:  "RecordExpr",
						Elems: elems,
					},
				},
			},
		}
		seq = append(seq, dag.NewValues(e))
	}
	return seq
}

func exprMatch(e dag.Expr, exprs []exprloc) int {
	target := zfmt.DAGExpr(e)
	for which, e := range exprs {
		if target == zfmt.DAGExpr(e.expr) {
			return which
		}
	}
	return -1
}

func (a *analyzer) semSelectFrom(loc ast.Loc, from *ast.From, seq dag.Seq) (dag.Seq, schema) {
	if from == nil {
		return seq, &dynamicSchema{}
	}
	off := len(seq)
	hasParent := off > 0
	seq, sch := a.semFrom(from, seq)
	if off >= len(seq) {
		// The chain didn't get lengthed so semFrom must have enocounteded
		// an error...
		return seq, nil
	}
	// If we have parents with both a from and select, report an error but
	// only if it's not a RobotScan where the parent feeds the from operateor.
	if _, ok := seq[off].(*dag.RobotScan); !ok {
		if hasParent {
			a.error(loc, errors.New("SELECT cannot have both an embedded FROM clause and input from parents"))
			return append(seq, badOp()), nil
		}
	}
	return seq, sch
}

func (a *analyzer) semSelectValue(sel *ast.Select, sch schema, seq dag.Seq) (dag.Seq, schema) {
	if sel.GroupBy != nil {
		a.error(sel, errors.New("SELECT VALUE cannot be used with GROUP BY"))
		seq = append(seq, badOp())
	}
	if sel.Having != nil {
		a.error(sel, errors.New("SELECT VALUE cannot be used with HAVING"))
		seq = append(seq, badOp())
	}
	exprs := make([]dag.Expr, 0, len(sel.Selection.Args))
	for _, as := range sel.Selection.Args {
		if as.Label != nil {
			a.error(sel, errors.New("SELECT VALUE cannot have AS clause in selection"))
		}
		var e dag.Expr
		if as.Expr == nil {
			e = &dag.This{Kind: "This"}
		} else {
			e = a.semExprSchema(sch, as.Expr)
		}
		exprs = append(exprs, e)
	}
	if sel.Where != nil {
		seq = append(seq, dag.NewFilter(a.semExprSchema(sch, sel.Where)))
	}
	seq = append(seq, dag.NewValues(exprs...))
	if sel.Distinct {
		seq = a.genDistinct(pathOf("this"), seq)
	}
	return seq, &dynamicSchema{}
}

func (a *analyzer) semValues(values *ast.SQLValues, seq dag.Seq) (dag.Seq, schema) {
	var schema *super.TypeRecord
	exprs := make([]dag.Expr, 0, len(values.Exprs))
	sctx := super.NewContext()
	for _, astExpr := range values.Exprs {
		e := a.semExpr(astExpr)
		val, err := rungen.EvalAtCompileTime(sctx, e)
		if err != nil {
			a.error(astExpr, errors.New("expressions in values clause must be constant"))
			return seq, badSchema()
		}
		// Parser requires tuples in SQLValues expressions so these should
		// always record values.
		recType := super.TypeUnder(val.Type()).(*super.TypeRecord)
		if schema == nil {
			schema = recType
		} else if schema != recType {
			a.error(astExpr, errors.New("values clause must contain uniformly typed values"))
		}
		exprs = append(exprs, e)
	}
	if schema == nil {
		a.error(values, errors.New("values clause must contain uniformly typed values"))
		return seq, badSchema()
	}
	columns := make([]string, 0, len(schema.Fields))
	for _, f := range schema.Fields {
		columns = append(columns, f.Name)
	}
	seq = append(seq, dag.NewValues(exprs...))
	return seq, &staticSchema{columns: columns}
}

func (a *analyzer) genDistinct(e dag.Expr, seq dag.Seq) dag.Seq {
	return append(seq, &dag.Distinct{
		Kind: "Distinct",
		Expr: e,
	})
}

func (a *analyzer) semSQLPipe(op *ast.SQLPipe, seq dag.Seq, alias *ast.TableAlias) (dag.Seq, schema) {
	if len(op.Ops) == 1 && isSQLOp(op.Ops[0]) {
		seq, sch := a.semSQLOp(op.Ops[0], seq)
		outSeq, outSch, err := derefSchemaWithAlias(sch, alias, seq)
		if err != nil {
			a.error(op.Ops[0], err)
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
			a.error(alias, errors.New("cannot apply column aliases to dynamically typed data"))
		}
	}
	return a.semSeq(op.Ops), &dynamicSchema{name: name}
}

func derefSchemaAs(sch schema, table string, seq dag.Seq) (dag.Seq, schema) {
	e, sch := sch.deref(table)
	if e != nil {
		seq = valuesExpr(e, seq)
	}
	return seq, sch
}

func derefSchema(sch schema, seq dag.Seq) (dag.Seq, schema) {
	return derefSchemaAs(sch, "", seq)
}

func derefSchemaWithAlias(insch schema, alias *ast.TableAlias, inseq dag.Seq) (dag.Seq, schema, error) {
	var table string
	if alias != nil {
		table = alias.Name
	}
	seq, sch := derefSchemaAs(insch, table, inseq)
	if alias == nil || len(alias.Columns) == 0 {
		return seq, sch, nil
	}
	if sch, ok := sch.(*staticSchema); ok {
		return mapColumns(sch.columns, alias, seq)
	}
	return seq, sch, errors.New("cannot apply column aliases to dynamically typed data")
}

func mapColumns(in []string, alias *ast.TableAlias, seq dag.Seq) (dag.Seq, schema, error) {
	if len(alias.Columns) > len(in) {
		return nil, nil, fmt.Errorf("cannot apply %d column aliases in table alias %q to table with %d columns", len(alias.Columns), alias.Name, len(in))
	}
	out := idsToStrings(alias.Columns)
	if !slices.Equal(in, out) {
		// Make a record expression...
		elems := make([]dag.RecordElem, 0, len(in))
		for k := range out {
			elems = append(elems, &dag.Field{
				Kind:  "Field",
				Name:  out[k],
				Value: &dag.This{Kind: "This", Path: []string{in[k]}},
			})
		}
		seq = valuesExpr(&dag.RecordExpr{
			Kind:  "RecordExpr",
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
	case *ast.Select, *ast.SQLLimitOffset, *ast.OrderBy, *ast.SQLPipe, *ast.SQLJoin, *ast.SQLValues:
		return true
	}
	return false
}

func (a *analyzer) semSQLOp(op ast.Op, seq dag.Seq) (dag.Seq, schema) {
	switch op := op.(type) {
	case *ast.SQLPipe:
		return a.semSQLPipe(op, seq, nil) //XXX should alias hang off SQLPipe?
	case *ast.Select:
		return a.semSelect(op, seq)
	case *ast.SQLValues:
		return a.semValues(op, seq)
	case *ast.SQLJoin:
		return a.semSQLJoin(op, seq)
	case *ast.OrderBy:
		out, schema := a.semSQLOp(op.Op, seq)
		var exprs []dag.SortExpr
		for _, e := range op.Exprs {
			exprs = append(exprs, a.semSortExpr(schema, e, false))
		}
		return append(out, &dag.Sort{Kind: "Sort", Exprs: exprs}), schema
	case *ast.SQLLimitOffset:
		out, schema := a.semSQLOp(op.Op, seq)
		if op.Offset != nil {
			out = append(out, &dag.Skip{Kind: "Skip", Count: a.evalPositiveInteger(op.Offset)})
		}
		if op.Limit != nil {
			out = append(out, &dag.Head{Kind: "Head", Count: a.evalPositiveInteger(op.Limit)})
		}
		return out, schema
	case *ast.With:
		if op.Recursive {
			a.error(op, errors.New("recursive WITH queries not currently supported"))
		}
		old := a.scope.ctes
		a.scope.ctes = maps.Clone(a.scope.ctes)
		defer func() { a.scope.ctes = old }()
		for _, c := range op.CTEs {
			// XXX Materialized option not currently supported.
			name := strings.ToLower(c.Name.Name)
			if _, ok := a.scope.ctes[name]; ok {
				a.error(c.Name, errors.New("duplicate WITH clause name"))
			}
			seq, schema := a.semSQLPipe(c.Body, nil, &ast.TableAlias{Name: c.Name.Name})
			a.scope.ctes[name] = &cte{seq, schema}
		}
		return a.semSQLOp(op.Body, seq)
	default:
		panic(fmt.Sprintf("semSQLOp: unknown op: %#v", op))
	}
}

func (a *analyzer) semCrossJoin(join *ast.CrossJoin, seq dag.Seq) (dag.Seq, schema) {
	if len(seq) > 0 {
		// At some point we might want to let parent data flow into a join somehow,
		// but for now we flag an error.
		a.error(join, errors.New("SQL cross join cannot inherit data from pipeline parent"))
	}
	leftSeq, leftSchema := a.semFromElem(join.Left, nil)
	rightSeq, rightSchema := a.semFromElem(join.Right, nil)
	sch := &joinSchema{left: leftSchema, right: rightSchema}
	par := &dag.Fork{
		Kind:  "Fork",
		Paths: []dag.Seq{leftSeq, rightSeq},
	}
	dagJoin := &dag.Join{
		Kind:       "Join",
		Style:      "cross",
		LeftAlias:  "left",
		RightAlias: "right",
	}
	return dag.Seq{par, dagJoin}, sch
}

// For now, each joining table is on the right...
// We don't have logic to not care about the side of the JOIN ON keys...
func (a *analyzer) semSQLJoin(join *ast.SQLJoin, seq dag.Seq) (dag.Seq, schema) {
	if len(seq) > 0 {
		// At some point we might want to let parent data flow into a join somehow,
		// but for now we flag an error.
		a.error(join, errors.New("SQL join cannot inherit data from pipeline parent"))
	}
	leftSeq, leftSchema := a.semFromElem(join.Left, nil)
	rightSeq, rightSchema := a.semFromElem(join.Right, nil)
	sch := &joinSchema{left: leftSchema, right: rightSchema}
	leftKey, rightKey, err := a.semSQLJoinCond(join.Cond, sch)
	if err != nil {
		// Join expression errors are already logged so suppress further notice.
		if err != badJoinCond {
			a.error(join.Cond, err)
		}
		return append(seq, badOp()), badSchema()
	}
	par := &dag.Fork{
		Kind:  "Fork",
		Paths: []dag.Seq{leftSeq, rightSeq},
	}
	dagJoin := &dag.Join{
		Kind:       "Join",
		Style:      join.Style,
		LeftAlias:  "left",
		LeftDir:    order.Unknown,
		LeftKey:    leftKey,
		RightAlias: "right",
		RightDir:   order.Unknown,
		RightKey:   rightKey,
	}
	return dag.Seq{par, dagJoin}, sch
}

var badJoinCond = errors.New("bad join condition")

func (a *analyzer) semSQLJoinCond(cond ast.JoinExpr, sch *joinSchema) (*dag.This, *dag.This, error) {
	//XXX we currently require field expressions for SQL joins and will need them
	// to resolve names to join side when we add scope tracking
	saved := a.scope.schema
	defer func() {
		a.scope.schema = saved
	}()
	a.scope.schema = sch
	l, r, err := a.semJoinCond(cond, "left", "right")
	if err != nil {
		return nil, nil, err
	}
	left, err := joinFieldAsThis(l)
	if err != nil {
		return nil, nil, err
	}
	right, err := joinFieldAsThis(r)
	return left, right, err
}

func joinFieldAsThis(e dag.Expr) (*dag.This, error) {
	if _, ok := e.(*dag.BadExpr); ok {
		return nil, badJoinCond
	}
	this, ok := e.(*dag.This)
	if !ok {
		return nil, errors.New("join condition must be equijoin on fields")
	}
	if len(this.Path) == 0 {
		return nil, errors.New("join expression cannot refer to 'this'")
	}
	return this, nil
}

func (a *analyzer) semJoinCond(cond ast.JoinExpr, leftAlias, rightAlias string) (dag.Expr, dag.Expr, error) {
	switch cond := cond.(type) {
	case *ast.JoinOnExpr:
		if id, ok := cond.Expr.(*ast.ID); ok {
			return a.semJoinCond(&ast.JoinUsingExpr{Fields: []ast.Expr{id}}, leftAlias, rightAlias)
		}
		binary, ok := cond.Expr.(*ast.BinaryExpr)
		if !ok || !(binary.Op == "==" || binary.Op == "=") {
			return nil, nil, errors.New("only equijoins currently supported")
		}
		leftKey, rightKey := a.semJoinOnExpr(cond, leftAlias, rightAlias, binary.LHS, binary.RHS)
		return leftKey, rightKey, nil
	case *ast.JoinUsingExpr:
		if len(cond.Fields) > 1 {
			return nil, nil, errors.New("join using currently limited to a single field")
		}
		if a.scope.schema != nil {
			sch := a.scope.schema.(*joinSchema)
			a.scope.schema = &joinUsingSchema{sch}
			defer func() { a.scope.schema = sch }()
		}
		e := a.semField(cond.Fields[0])
		key, ok := e.(*dag.This)
		if !ok {
			return e, e, nil
		}
		return key, key, nil
	default:
		panic(fmt.Sprintf("semJoinCond: unknown type: %T", cond))
	}
}

func (a *analyzer) semJoinOnExpr(cond *ast.JoinOnExpr, leftAlias, rightAlias string, lhs, rhs ast.Expr) (dag.Expr, dag.Expr) {
	var isbad bool
	var left, right dag.Expr
	for _, in := range []ast.Expr{lhs, rhs} {
		e, alias := a.semEquiJoinExpr(in)
		if _, ok := e.(*dag.BadExpr); ok {
			isbad = true
			continue
		}
		switch alias {
		case leftAlias:
			if left != nil {
				a.error(cond.Expr, errors.New("self joins not currently supported"))
			}
			left = e
		case rightAlias:
			if right != nil {
				a.error(cond.Expr, errors.New("self joins not currently supported"))
			}
			right = e
		default:
			a.error(in, fmt.Errorf("ambiguous field name %q", alias))
		}
	}
	if isbad {
		return badExpr(), badExpr()
	}
	return left, right
}

// Possible error states:
//   - No field reference in one side of join expression (allowed for cross join
//     but we ain't there).
//   - There's an ID that doesn't have an alias (i.e., len < 2)
//   - Mix of aliases in one side of equi-join (allowed for cross join).
func (a *analyzer) semEquiJoinExpr(in ast.Expr) (dag.Expr, string) {
	e := a.semExpr(in)
	if _, ok := e.(*dag.BadExpr); ok {
		return e, ""
	}
	fields := fieldsInExpr(e)
	if len(fields) == 0 {
		a.error(in, errors.New("no field references in join expression"))
		return badExpr(), ""
	}
	var aliases []string
	for _, field := range fields {
		if len(field.Path) == 0 {
			a.error(in, errors.New("cannot join on this"))
			return badExpr(), ""
		}
		aliases = append(aliases, field.Path[0])
	}
	if len(slices.Compact(aliases)) > 1 {
		a.error(in, errors.New("more than one alias referenced in one side of equi-join"))
		return badExpr(), ""
	}
	// Strip alias from expr.
	for _, field := range fields {
		field.Path = field.Path[1:]
	}
	return e, aliases[0]
}

func fieldsInExpr(e dag.Expr) []*dag.This {
	if this, ok := e.(*dag.This); ok {
		return []*dag.This{this}
	}
	var fields []*dag.This
	dag.WalkT(reflect.ValueOf(e), func(field *dag.This) *dag.This {
		fields = append(fields, field)
		return field
	})
	return fields
}

type exprloc struct {
	expr dag.Expr
	loc  ast.Expr
}

func (a *analyzer) semGroupBy(sch *selectSchema, in []ast.Expr) []exprloc {
	var out []exprloc
	var funcs aggfuncs
	for k, expr := range in {
		e := a.semExprSchema(sch, expr)
		if which, ok := isOrdinal(a.sctx, e); ok {
			var err error
			if e, err = sch.resolveOrdinal(which); err != nil {
				a.error(in[k], err)
			}
		}
		// Grouping expressions can't have agg funcs so we parse as a column
		// and see if any agg functions were found.
		c, _ := newColumn("", in[k], e, &funcs)
		if c != nil && c.isAgg {
			a.error(in[k], errors.New("aggregate function cannot appear in GROUP BY clause"))
		}
		out = append(out, exprloc{e, expr})
	}
	return out
}

func isOrdinal(sctx *super.Context, e dag.Expr) (int, bool) {
	if literal, ok := e.(*dag.Literal); ok {
		v := sup.MustParseValue(sctx, literal.Value)
		return int(v.AsInt()), super.IsInteger(v.Type().ID())
	}
	return -1, false
}

func (a *analyzer) semProjection(sch *selectSchema, args []ast.AsExpr, funcs *aggfuncs) projection {
	out := &staticSchema{}
	sch.out = out
	labels := make(map[string]struct{})
	var proj projection
	for _, as := range args {
		if isStar(as) {
			proj = append(proj, column{})
			continue
		}
		col := a.semAs(sch, as, funcs)
		if as.Label != nil {
			if _, ok := labels[col.name]; ok {
				a.error(as.Label, fmt.Errorf("duplicate column label %q", col.name))
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

func isStar(a ast.AsExpr) bool {
	return a.Expr == nil && a.Label == nil
}

func (a *analyzer) semAs(sch schema, as ast.AsExpr, funcs *aggfuncs) *column {
	e := a.semExprSchema(sch, as.Expr)
	// If we have a name from an AS clause, use it. Otherwise, infer a name.
	var name string
	if as.Label != nil {
		name = as.Label.Name
		if name == "" {
			a.error(as.Label, errors.New("label cannot be an empty string"))
		}
	} else {
		name = inferColumnName(e, as.Expr)
	}
	c, err := newColumn(name, as.Expr, e, funcs)
	if err != nil {
		a.error(as, err)
	}
	return c
}

func (a *analyzer) semExprSchema(s schema, e ast.Expr) dag.Expr {
	save := a.scope.schema
	a.scope.schema = s
	out := a.semExpr(e)
	a.scope.schema = save
	return out
}

// inferColumnName translates an expression to a column name.
// If it's a dotted field path, we use the last element of the path.
// Otherwise, we format the expression as text.  Pretty gross but
// that's what SQL does!  And it seems different implementations format
// expressions differently.  XXX we need to check ANSI SQL spec here
func inferColumnName(e dag.Expr, ae ast.Expr) string {
	switch e := e.(type) {
	case *dag.This:
		return field.Path(e.Path).Leaf()
	default:
		return zfmt.ASTExpr(ae)
	}
}

func valuesExpr(e dag.Expr, seq dag.Seq) dag.Seq {
	return append(seq, dag.NewValues(e))
}

func wrapThis(field string) *dag.RecordExpr {
	return wrapField(field, &dag.This{Kind: "This"})
}

func wrapField(field string, e dag.Expr) *dag.RecordExpr {
	return &dag.RecordExpr{
		Kind: "RecordExpr",
		Elems: []dag.RecordElem{
			&dag.Field{
				Kind:  "Field",
				Name:  field,
				Value: e,
			},
		},
	}
}
