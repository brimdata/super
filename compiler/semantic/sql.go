package semantic

import (
	"errors"
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/compiler/kernel"
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
	seq = yieldExpr(wrapThis("in"), seq)
	if len(funcs) != 0 || len(keyExprs) != 0 {
		seq = a.genAggregate(sel.Loc, proj, where, keyExprs, funcs, having, seq)
		return seq, sch.out
	}
	if having != nil {
		a.error(sel.Having, errors.New("HAVING clause requires aggregations and/or a GROUP BY clause"))
	}
	seq = a.genYield(proj, where, sch, seq)
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

func (a *analyzer) genYield(proj projection, where dag.Expr, sch *selectSchema, seq dag.Seq) dag.Seq {
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
		seq = append(seq, &dag.Yield{
			Kind:  "Yield",
			Exprs: []dag.Expr{e},
		})
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
		seq = a.genYield(proj, nil, nil, seq)
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
	seq = yieldExpr(wrapThis("in"), seq)
	seq = a.genAggregateOutput(proj, keyExprs, seq)
	if having != nil {
		seq = append(seq, dag.NewFilter(having))
	}
	return yieldExpr(pathOf("out"), seq)
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
		if col.isAgg {
			elems = append(elems, &dag.Field{
				Kind:  "Field",
				Name:  col.name,
				Value: col.expr,
			})
		} else {
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
				if which < 0 {
					a.error(col.loc, fmt.Errorf("no corresponding grouping element for non-aggregate %q", col.name))
				}
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
		seq = append(seq, &dag.Yield{
			Kind:  "Yield",
			Exprs: []dag.Expr{e},
		})
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
			a.error(loc, errors.New("SELECT cannot have both an embedded FROM claue and input from parents"))
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
		if as.ID != nil {
			a.error(sel, errors.New("SELECT VALUE cannot have AS clause in selection"))
		}
		exprs = append(exprs, a.semExprSchema(sch, as.Expr))
	}
	if sel.Where != nil {
		seq = append(seq, dag.NewFilter(a.semExprSchema(sch, sel.Where)))
	}
	seq = append(seq, &dag.Yield{
		Kind:  "Yield",
		Exprs: exprs,
	})
	if sel.Distinct {
		seq = a.genDistinct(pathOf("this"), seq)
	}
	return seq, &dynamicSchema{}
}

func (a *analyzer) genDistinct(e dag.Expr, seq dag.Seq) dag.Seq {
	return append(seq, &dag.Distinct{
		Kind: "Distinct",
		Expr: e,
	})
}

func (a *analyzer) semSQLPipe(op *ast.SQLPipe, seq dag.Seq, alias string) (dag.Seq, schema) {
	if len(op.Ops) == 1 && isSQLOp(op.Ops[0]) {
		seq, sch := a.semSQLOp(op.Ops[0], seq)
		return derefSchema(sch, alias, seq)
	}
	if len(seq) > 0 {
		panic("semSQLOp: SQL pipes can't have parents")
	}
	return a.semSeq(op.Ops), &dynamicSchema{name: alias}
}

func derefSchema(sch schema, alias string, seq dag.Seq) (dag.Seq, schema) {
	e, sch := sch.deref(alias)
	if e != nil {
		seq = yieldExpr(e, seq)
	}
	return seq, sch
}

func isSQLOp(op ast.Op) bool {
	switch op.(type) {
	case *ast.Select, *ast.Limit, *ast.OrderBy, *ast.SQLPipe, *ast.SQLJoin:
		return true
	}
	return false
}

func (a *analyzer) semSQLOp(op ast.Op, seq dag.Seq) (dag.Seq, schema) {
	switch op := op.(type) {
	case *ast.SQLPipe:
		return a.semSQLPipe(op, seq, "") //XXX empty string for alias?
	case *ast.Select:
		return a.semSelect(op, seq)
	case *ast.SQLJoin:
		return a.semSQLJoin(op, seq)
	case *ast.OrderBy:
		nullsFirst, ok := nullsFirst(op.Exprs)
		if !ok {
			a.error(op, errors.New("differring nulls first/last clauses not yet supported"))
			return append(seq, badOp()), badSchema()
		}
		out, schema := a.semSQLOp(op.Op, seq)
		var exprs []dag.SortExpr
		for _, e := range op.Exprs {
			exprs = append(exprs, a.semSortExpr(schema, e))
		}
		return append(out, &dag.Sort{
			Kind:       "Sort",
			Args:       exprs,
			NullsFirst: nullsFirst,
			Reverse:    false, //XXX this should go away
		}), schema
	case *ast.Limit:
		e := a.semExpr(op.Count)
		var err error
		val, err := kernel.EvalAtCompileTime(a.sctx, e)
		if err != nil {
			a.error(op.Count, err)
			return append(seq, badOp()), badSchema()
		}
		if !super.IsInteger(val.Type().ID()) {
			a.error(op.Count, fmt.Errorf("expression value must be an integer value: %s", sup.FormatValue(val)))
			return append(seq, badOp()), badSchema()
		}
		limit := val.AsInt()
		if limit < 1 {
			a.error(op.Count, errors.New("expression value must be a positive integer"))
		}
		head := &dag.Head{
			Kind:  "Head",
			Count: int(limit),
		}
		out, schema := a.semSQLOp(op.Op, seq)
		return append(out, head), schema
	default:
		panic(fmt.Sprintf("semSQLOp: unknown op: %#v", op))
	}
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
	leftSeq = yieldExpr(wrapThis("left"), leftSeq)
	rightSeq, rightSchema := a.semFromElem(join.Right, nil)
	rightSeq = yieldExpr(wrapThis("right"), rightSeq)
	sch := &joinSchema{left: leftSchema, right: rightSchema}
	leftKey, rightKey, err := a.semSQLJoinCond(join.Cond, sch)
	if err != nil {
		// Join expression errors are already logged so suppress further notice.
		if err != badJoinCond {
			a.error(join.Cond, err)
		}
		return append(seq, badOp()), badSchema()
	}
	assignment := dag.Assignment{
		Kind: "Assignment",
		LHS:  pathOf("right"),
		RHS:  pathOf("right"),
	}
	par := &dag.Fork{
		Kind:  "Fork",
		Paths: []dag.Seq{{dag.PassOp}, rightSeq},
	}
	dagJoin := &dag.Join{
		Kind:     "Join",
		Style:    join.Style,
		LeftDir:  order.Unknown,
		LeftKey:  leftKey,
		RightDir: order.Unknown,
		RightKey: rightKey,
		Args:     []dag.Assignment{assignment},
	}
	return append(append(leftSeq, par), dagJoin), sch
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
	l, r, err := a.semJoinCond(cond)
	if err != nil {
		return nil, nil, err
	}
	left, err := joinFieldAsThis("left", l)
	if err != nil {
		return nil, nil, err
	}
	right, err := joinFieldAsThis("right", r)
	return left, right, err
}

func joinFieldAsThis(which string, e dag.Expr) (*dag.This, error) {
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
	if this.Path[0] != which {
		return nil, fmt.Errorf("%s-hand side of join condition must refer to %s-hand table", which, which)
	}
	return this, nil
}

func (a *analyzer) semJoinCond(cond ast.JoinExpr) (dag.Expr, dag.Expr, error) {
	switch cond := cond.(type) {
	case *ast.JoinOnExpr:
		if id, ok := cond.Expr.(*ast.ID); ok {
			return a.semJoinCond(&ast.JoinUsingExpr{Fields: []ast.Expr{id}})
		}
		binary, ok := cond.Expr.(*ast.BinaryExpr)
		if !ok || !(binary.Op == "==" || binary.Op == "=") {
			return nil, nil, errors.New("only equijoins currently supported")
		}
		leftKey := a.semExpr(binary.LHS)
		rightKey := a.semExpr(binary.RHS)
		return leftKey, rightKey, nil
	case *ast.JoinUsingExpr:
		if len(cond.Fields) > 1 {
			return nil, nil, errors.New("join using currently limited to a single field")
		}
		key, ok := a.semField(cond.Fields[0]).(*dag.This)
		if !ok {
			return nil, nil, errors.New("join using key must be a field reference")
		}
		return key, key, nil
	default:
		panic(fmt.Sprintf("semJoinCond: unknown type: %T", cond))
	}
}

func nullsFirst(exprs []ast.SortExpr) (bool, bool) {
	if len(exprs) == 0 {
		panic("nullsFirst()")
	}
	if !hasNullsFirst(exprs) {
		return false, true
	}
	// If the nulls firsts are all the same, then we can use
	// nullsfirst; otherwise, if they differ, the runtime currently
	// can't support it.
	for _, e := range exprs {
		if e.Nulls == nil || e.Nulls.Name != "first" {
			return false, false
		}
	}
	return true, true
}

func hasNullsFirst(exprs []ast.SortExpr) bool {
	for _, e := range exprs {
		if e.Nulls != nil && e.Nulls.Name == "first" {
			return true
		}
	}
	return false
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

func (a *analyzer) semProjection(sch *selectSchema, args []ast.AsExpr, funcs *aggfuncs) projection {
	out := &anonSchema{}
	sch.out = out
	conflict := make(map[string]struct{})
	var proj projection
	for _, as := range args {
		if isStar(as) {
			proj = append(proj, column{})
			continue
		}
		col := a.semAs(sch, as, funcs)
		//XXX check for conflict for now, but we're getting rid of this soon
		if _, ok := conflict[col.name]; ok {
			a.error(as.ID, fmt.Errorf("%q: conflicting name in projection; try an AS clause", col.name))
		}
		proj = append(proj, *col)
		out.columns = append(out.columns, col.name)
	}
	return proj
}

func isStar(a ast.AsExpr) bool {
	return a.Expr == nil && a.ID == nil
}

func (a *analyzer) semAs(sch schema, as ast.AsExpr, funcs *aggfuncs) *column {
	e := a.semExprSchema(sch, as.Expr)
	// If we have a name from an AS clause, use it.  Otherwise,
	// infer a name.
	var name string
	if as.ID != nil {
		name = as.ID.Name
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
	path, err := deriveLHSPath(e)
	if err != nil {
		return zfmt.ASTExpr(ae)
	}
	return field.Path(path).Leaf()
}

func yieldExpr(e dag.Expr, seq dag.Seq) dag.Seq {
	return append(seq, &dag.Yield{
		Kind:  "Yield",
		Exprs: []dag.Expr{e},
	})
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
