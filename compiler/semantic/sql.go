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
	"github.com/brimdata/super/zfmt"
	"github.com/brimdata/super/zson"
)

// Analyze a SQL select expression which may have arbitrary nested subqueries
// and may or may not have its sources embedded.
// The output of a select expression is a record that wraps its input and it's
// selected columns in a record {in:any,out:any}.  The schema returned represents
// the observable scope of the selected elements.  When the parent operator is
// an OrderBy, it can reach into the "in" part of the select scope (for non-aggregates)
// and also sort by the out elements.  It's up to the caller to unwrap the in/out
// record when returning to pipeline context.
func (a *analyzer) semSelect(sel *ast.Select, seq dag.Seq) (dag.Seq, schema) {
	var selSchema schemaSelect
	//XXX if we hit a lateral join in the from clause we need to refer back to this schema
	if sel.From != nil {
		off := len(seq)
		hasParent := off > 0
		var from schema
		seq, from = a.semFrom(sel.From, seq)
		if off >= len(seq) {
			// The chain didn't get lengthed so semFrom must have enocounteded
			// an error...
			return seq, badSchema()
		}
		// If we have parents with both a from and select, report an error but
		// only if it's not a RobotScan where the parent feeds the from operateor.
		if _, ok := seq[off].(*dag.RobotScan); !ok {
			if hasParent {
				a.error(sel, errors.New("SELECT cannot have both an embedded FROM claue and input from parents"))
				return append(seq, badOp()), badSchema()
			}
		}
		selSchema.in = from
	} else {
		// If there's no from clause, presume a dynamic schema.
		// XXX we need to handle recursive cases where the select is a
		// correlated subquery.
		// XXX we should figure out if this is a null input and set up a null schema?
		selSchema.in = &schemaDynamic{}
	}
	a.enterScopeWithSchema(&selSchema)
	defer a.exitScope()
	if sel.Value {
		return a.semSelectValue(sel, selSchema, seq)
	}
	proj, ok := a.semProjection(sel.Selection.Args)
	if !ok {
		return dag.Seq{badOp()}, badSchema()
	}
	// XXX form out schema here from proj
	if sel.Where != nil {
		seq = append(seq, dag.NewFilter(a.semExpr(sel.Where)))
	}
	if sel.GroupBy != nil {
		if proj.hasStar() {
			a.error(sel, errors.New("aggregate mixed with *-selector not yet supported"))
			return append(seq, badOp()), badSchema()
		}
		seq, ok = a.semGroupBy(sel.GroupBy, proj, seq)
		if !ok {
			return seq, badSchema()
		}
		//XXX need having schema... can have mix of agg output and unselected group-by keys
		// maybe just another in/out as order can also reach into unselected group-by keys
		if sel.Having != nil {
			seq = append(seq, dag.NewFilter(a.semExpr(sel.Having)))
		}
	} else if sel.Selection.Args != nil {
		if sel.Having != nil {
			a.error(sel.Having, errors.New("HAVING clause used without GROUP BY"))
			return append(seq, badOp()), badSchema()
		}
		seq = a.convertProjection(sel.Selection.Loc, proj, seq)
	}
	//XXX hmm, this needs to operate on out
	if sel.Distinct {
		seq = a.semDistinct(seq)
	}
	return seq, &selSchema
}

func (a *analyzer) semSelectValue(sel *ast.Select, schema schemaSelect, seq dag.Seq) (dag.Seq, schema) {
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
		exprs = append(exprs, a.semExpr(as.Expr))
	}
	seq = append(seq, &dag.Yield{
		Kind:  "Yield",
		Exprs: exprs,
	})
	out := &schemaSelect{in: schema.in, out: &schemaDynamic{}}
	//XXX need in/out here
	if sel.Where != nil {
		seq = append(seq, dag.NewFilter(a.semExpr(sel.Where)))
	}
	//XXX hmm, this needs to operate on out
	if sel.Distinct {
		seq = a.semDistinct(seq)
	}
	return seq, out
}

func (a *analyzer) semDistinct(seq dag.Seq) dag.Seq {
	seq = append(seq, &dag.Sort{
		Kind: "Sort",
		Args: []dag.SortExpr{
			{
				Key:   &dag.This{Kind: "This"},
				Order: order.Asc,
			},
		},
	})
	return append(seq, &dag.Uniq{
		Kind: "Uniq",
	})
}

func (a *analyzer) semSQLPipe(op *ast.SQLPipe, seq dag.Seq) (dag.Seq, schema) {
	if len(op.Ops) == 1 && isSQLOp(op.Ops[0]) {
		return a.semSQLOp(op.Ops[0], seq)
	}
	if len(seq) > 0 {
		panic("semSQLOp: SQL pipes can't have parents")
	}
	return a.semSeq(op.Ops), &schemaDynamic{}
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
		return a.semSQLPipe(op, seq)
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
		var exprs []dag.SortExpr
		for _, e := range op.Exprs {
			exprs = append(exprs, a.semSortExpr(e))
		}
		out, schema := a.semSQLOp(op.Op, seq)
		return append(out, &dag.Sort{
			Kind:       "Sort",
			Args:       exprs,
			NullsFirst: nullsFirst,
			Reverse:    false, //XXX this should go away
		}), schema
	case *ast.Limit:
		e := a.semExpr(op.Count)
		var err error
		val, err := kernel.EvalAtCompileTime(a.zctx, e)
		if err != nil {
			a.error(op.Count, err)
			return append(seq, badOp()), badSchema()
		}
		if !super.IsInteger(val.Type().ID()) {
			a.error(op.Count, fmt.Errorf("expression value must be an integer value: %s", zson.FormatValue(val)))
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
	// XXX  For now we require an alias on the
	// right side and combine the entire right side value into the row
	// using the existing join semantics of assignment where the lval
	// lives in the left record and the rval comes from the right.
	if join.Right.Alias == nil {
		a.error(join.Right, errors.New("SQL joins currently require a table alias on the right lef of the join"))
		seq = append(seq, badOp())
	}
	leftKey, rightKey, err := a.semSQLJoinCond(join.Cond)
	if err != nil {
		a.error(join.Cond, errors.New("SQL joins currently limited to equijoin on fields"))
		return append(seq, badOp()), badSchema()
	}
	if len(seq) > 0 {
		// At some point we might want to let parent data flow into a join somehow,
		// but for now we flag an error.
		a.error(join, errors.New("SQL join cannot inherit data from pipeline parent"))
	}
	leftPath, leftSchema := a.semFromElem(join.Left, nil)
	rightPath, rightSchema := a.semFromElem(join.Right, nil)
	alias := join.Right.Alias.Text
	assignment := dag.Assignment{
		Kind: "Assignment",
		LHS:  pathOf(alias),
		RHS:  &dag.This{Kind: "This", Path: field.Path{alias}},
	}
	par := &dag.Fork{
		Kind:  "Fork",
		Paths: []dag.Seq{{dag.PassOp}, rightPath},
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
	seq = leftPath
	seq = append(seq, par)
	return append(seq, dagJoin), &schemaJoin{left: leftSchema, right: rightSchema}
}

func (a *analyzer) semSQLJoinCond(cond ast.JoinExpr) (*dag.This, *dag.This, error) {
	//XXX we currently require field expressions for SQL joins and will need them
	// to resolve names to join side when we add scope tracking
	l, r, err := a.semJoinCond(cond)
	if err != nil {
		return nil, nil, err
	}
	left, ok := l.(*dag.This)
	if !ok {
		return nil, nil, errors.New("join keys must be field references")
	}
	right, ok := r.(*dag.This)
	if !ok {
		return nil, nil, errors.New("join keys must be field references")
	}
	return left, right, nil
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

func (a *analyzer) convertProjection(loc ast.Node, proj projection, seq dag.Seq) dag.Seq {
	// This is a straight select without a group-by.
	// If all the expressions are aggregators, then we build a group-by.
	// If it's mixed, we return an error.  Otherwise, we yield a record.
	var nagg int
	for _, p := range proj {
		if p.agg != nil {
			nagg++
		}
	}
	if nagg == 0 {
		return proj.yieldScalars(seq)
	}
	if nagg != len(proj) {
		a.error(loc, errors.New("cannot mix aggregations and non-aggregations without a GROUP BY"))
		return seq
	}
	// This projection has agg funcs but no group-by keys and we've
	// confirmed that all the columns are agg funcs, so build a simple
	// Summarize operator without group-by keys.
	var assignments []dag.Assignment
	for _, col := range proj {
		a := dag.Assignment{
			Kind: "Assignment",
			LHS:  &dag.This{Kind: "This", Path: field.Path{col.name}},
			RHS:  col.agg,
		}
		assignments = append(assignments, a)
	}
	return append(seq, &dag.Summarize{
		Kind: "Summarize",
		Aggs: assignments,
	})
}

func (a *analyzer) semGroupBy(exprs []ast.Expr, proj projection, seq dag.Seq) (dag.Seq, bool) {
	// Unlike the original zed runtime, SQL group-by elements do not have explicit
	// keys and may just be a single identifier or an expression.  We don't quite
	// capture the correct scoping here but this is a start before we implement
	// more sophisticated scoping and identifier bindings.  For our binding-in-the-data
	// approach, we can create temp fields for unnamed group-by expressions and
	// drop them on exit from the scope.  For now, we allow only path expressions
	// and match them with equivalent path expressions in the selection.
	var paths field.List
	for _, e := range exprs {
		this, ok := a.semGroupByKey(e)
		if !ok {
			return nil, false
		}
		paths = append(paths, this.Path)
	}
	// Make sure all scalars are in the group-by keys.
	scalars := proj.scalars()
	for k, col := range scalars {
		path := col.scalar
		if this, ok := col.scalar.(*dag.This); ok {
			if field.Path(this.Path).In(paths) {
				continue
			}
		}
		if !(field.Path{col.name}).In(paths) {
			a.error(exprs[k], fmt.Errorf("'%s': selected expression is missing from GROUP BY clause (and is not an aggregation)", path))
			return nil, false
		}
	}
	// Now that the selection and keys have been checked, build the
	// key expressions from the scalars of the select and build the
	// aggregators from the aggregation functions present in the select clause.
	var keyExprs []dag.Assignment
	for _, col := range scalars {
		keyExprs = append(keyExprs, dag.Assignment{
			Kind: "Assignment",
			LHS:  &dag.This{Kind: "This", Path: field.Path{col.name}},
			RHS:  col.scalar,
		})
	}
	var aggExprs []dag.Assignment
	for _, col := range proj.aggs() {
		aggExprs = append(aggExprs, dag.Assignment{
			Kind: "Assignment",
			LHS:  &dag.This{Kind: "This", Path: field.Path{col.name}},
			RHS:  col.agg,
		})
	}
	return append(seq, &dag.Summarize{
		Kind: "Summarize",
		Keys: keyExprs,
		Aggs: aggExprs,
	}), true
}

func (a *analyzer) semProjection(args []ast.AsExpr) (projection, bool) {
	conflict := make(map[string]struct{})
	var proj projection
	for _, as := range args {
		if isStar(as) {
			proj = append(proj, column{})
			continue
		}
		col, ok := a.semAs(as)
		if !ok {
			return nil, false
		}
		if _, ok := conflict[col.name]; ok {
			a.error(as.ID, fmt.Errorf("%q: conflicting name in projection; try an AS clause", col.name))
			return nil, false
		}
		proj = append(proj, col)
	}
	return proj, true
}

func (a *analyzer) semAs(as ast.AsExpr) (column, bool) {
	e := a.semExpr(as.Expr)
	// If we have a name from an AS clause, use it.  Otherwise,
	// infer a name.
	var name string
	if as.ID != nil {
		name = as.ID.Name
	} else {
		name = inferColumnName(e)
	}
	// We currently recognize only agg funcs that are top level.
	// This means expressions with embedded agg funcs will turn
	// into streaming aggs, which is not what we want, but we will
	// address this later. XXX
	if agg, ok := e.(*dag.Agg); ok {
		// The name here was already pulled out of the Agg by inference above.
		return column{name: name, agg: agg}, true
	}
	return column{name: name, scalar: e}, true
}

// inferColumnName translates an expression to a column name.
// If it's a dotted field path, we use the last element of the path.
// Otherwise, we format the expression as text.  Pretty gross but
// that's what SQL does!  And it seems different implementations format
// expressions differently.  XXX we need to check ANSI SQL spec here
func inferColumnName(e dag.Expr) string {
	path, err := deriveLHSPath(e)
	if err != nil {
		return zfmt.DAGExpr(e)
	}
	return field.Path(path).Leaf()
}

func (a *analyzer) semGroupByKey(in ast.Expr) (*dag.This, bool) {
	e := a.semExpr(in)
	this, ok := e.(*dag.This)
	if !ok {
		a.error(in, errors.New("GROUP BY expressions are not yet supported"))
		return nil, false
	}
	if len(this.Path) == 0 {
		a.error(in, errors.New("cannot use 'this' as GROUP BY expression"))
		return nil, false
	}
	return this, true
}
