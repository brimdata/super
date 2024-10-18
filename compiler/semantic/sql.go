package semantic

import (
	"errors"
	"fmt"

	zed "github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/ast/dag"
	"github.com/brimdata/super/compiler/kernel"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/sam/expr/agg"
	"github.com/brimdata/super/zson"
)

// Analyze a SQL select expression which may have arbitrary nested subqueries
// and always has sources embedded.  Because data sources are explicit, we never
// have a parent operator of a select and thus a Seq is not passed in here.
func (a *analyzer) semSQLSelect(sel *ast.Select) dag.Seq {
	if sel.Distinct {
		a.error(sel, errors.New("SELECT DISTINCT not yet supported"))
		return dag.Seq{badOp()}
	}
	if sel.Value {
		return a.semSelectValue(sel)
	}
	selection, err := a.newSQLSelection(sel.Args)
	if err != nil {
		a.error(sel, err)
		return dag.Seq{badOp()}
	}
	var seq dag.Seq
	//XXX when from not present, need to check that select exprs are either constant
	// or refer to a from stream at an outer scope
	if sel.From != nil {
		seq = a.semSQLFrom(sel.From, seq)
	} else {
		//XXX we need to create a source from the select expression presuming
		// it's a constant value
		a.error(sel, errors.New("SELECT without a FROM claue not yet supported"))
		seq = append(seq, badOp())
	}
	if sel.Where != nil {
		seq = append(seq, dag.NewFilter(a.semExpr(sel.Where)))
	}
	if sel.GroupBy != nil {
		groupby, err := a.convertSQLGroupBy(sel.GroupBy, selection)
		if err != nil {
			a.error(sel, err)
			seq = append(seq, badOp())
		} else {
			seq = append(seq, groupby)
			if sel.Having != nil {
				seq = append(seq, dag.NewFilter(a.semExpr(sel.Having)))
			}
		}
	} else if sel.Args != nil {
		if sel.Having != nil {
			a.error(sel, errors.New("HAVING clause used without GROUP BY"))
			seq = append(seq, badOp())
		}
		// GroupBy will do the cutting but if there's no GroupBy,
		// then we need a cut for the select expressions.
		// For SELECT *, cutter is nil.
		selector, err := convertSQLSelect(selection)
		if err != nil {
			a.error(sel, err)
			seq = append(seq, badOp())
		} else {
			seq = append(seq, selector)
		}
	}
	if len(seq) == 0 {
		seq = []dag.Op{dag.PassOp}
	}
	return seq
}

func (a *analyzer) isImpliedSelectValue(assignments ast.Assignments) bool {
	if len(assignments) != 1 {
		return false
	}
	assignment := assignments[0]
	if v, _ := a.isAgg(assignment.RHS); v != nil {
		return false
	}
	return assignment.LHS == nil
}

func (a *analyzer) semSelectValue(sel *ast.Select) dag.Seq {
	var seq dag.Seq
	//XXX when from not present, need to check that select exprs are either constant
	// or refer to a from stream at an outer scope
	if sel.From != nil {
		seq = a.semSQLFrom(sel.From, seq)
	} else {
		//XXX we need to create a source from the select expression presuming
		// it's a constant value
		a.error(sel, errors.New("SELECT without a FROM claue not yet supported"))
		seq = append(seq, badOp())
	}
	if sel.Where != nil {
		seq = append(seq, dag.NewFilter(a.semExpr(sel.Where)))
	}
	if sel.GroupBy != nil {
		a.error(sel, errors.New("SELECT VALUE cannot be used with GROUP BY"))
		seq = append(seq, badOp())
	}
	if sel.Having != nil {
		a.error(sel, errors.New("SELECT VALUE cannot be used with HAVING"))
		seq = append(seq, badOp())
	}
	exprs := make([]dag.Expr, 0, len(sel.Args))
	for _, assignment := range sel.Args {
		if assignment.LHS != nil {
			a.error(sel, errors.New("SELECT VALUE cannot AS clause in selection"))
		}
		exprs = append(exprs, a.semExpr(assignment.RHS))
	}
	return append(seq, &dag.Yield{
		Kind:  "Yield",
		Exprs: exprs,
	})
}

func (a *analyzer) semSQLFrom(froms []ast.Op, seq dag.Seq) dag.Seq {
	if len(froms) > 1 {
		a.error(froms[1], errors.New("multiple FROM elements not yet supported"))
		return append(seq, badOp())
	}
	return a.semSQLOp(froms[0], seq)
}

func (a *analyzer) semSQLOp(op ast.Op, seq dag.Seq) dag.Seq {
	switch op := op.(type) {
	case *ast.Select:
		if len(seq) > 0 {
			panic("semSQLOp: select can't have parents")
		}
		return a.semSQLSelect(op)
	case *ast.Table:
		// For now assume a "table" is always an input file.  When we
		// add lake support here, we can have a syntactic way to differentiate
		// files, pools, and urls.
		return append(seq, &dag.FileScan{
			Kind: "FileScan",
			Path: op.Name,
			// Format:   s.Format, XXX we should have a way to specify it or infer?
			// SortKeys: a.semSortKeys(s.SortKeys), XXX should have way to hint order
		})
	case *ast.Alias:
		// Wrap the values in a single-field record with the name of the alias.
		seq = a.semSQLOp(op.Op, seq)
		return append(seq, &dag.Yield{
			Kind: "Yield",
			Exprs: []dag.Expr{
				&dag.RecordExpr{
					Kind: "RecordExpr",
					Elems: []dag.RecordElem{
						&dag.Field{
							Kind:  "Field",
							Name:  op.Name,
							Value: &dag.This{Kind: "This"},
						},
					},
				},
			},
		})
	case *ast.SQLJoin:
		return a.semSQLJoin(op, seq)
	case *ast.OrderBy:
		nullsFirst, ok := inferNullsFirst(op.Exprs)
		if !ok {
			a.error(op, errors.New("differring nulls first/last clauses not yet supported"))
			return append(seq, badOp())
		}
		var exprs []dag.SortExpr
		for _, e := range op.Exprs {
			exprs = append(exprs, a.semSortExpr(e))
		}
		return append(a.semSQLOp(op.Op, seq), &dag.Sort{
			Kind:       "Sort",
			Args:       exprs,
			NullsFirst: nullsFirst,
			Reverse:    false, //XXX this should go away
		})
	case *ast.Limit:
		e := a.semExpr(op.Count)
		var err error
		val, err := kernel.EvalAtCompileTime(a.zctx, e)
		if err != nil {
			a.error(op.Count, err)
			return append(seq, badOp())
		}
		if !zed.IsInteger(val.Type().ID()) {
			a.error(op.Count, fmt.Errorf("expression value must be an integer value: %s", zson.FormatValue(val)))
			return append(seq, badOp())
		}
		limit := val.AsInt()
		if limit < 1 {
			a.error(op.Count, errors.New("expression value must be a positive integer"))
		}
		head := &dag.Head{
			Kind:  "Head",
			Count: int(limit),
		}
		return append(a.semSQLOp(op.Op, seq), head)
	default:
		panic(fmt.Sprintf("semSQLOp: unknown op: %T", op))
	}
}

func inferNullsFirst(exprs []ast.SortExpr) (bool, bool) {
	//XXX figure out if there's a single nullsfirt that works for
	// all the sort expressions
	if len(exprs) == 1 {
		if nulls := exprs[0].Nulls; nulls != nil {
			return nulls.Name == "first", true
		}
		return false, true
	}
	panic("inferNullsFirst: TBD")
}

func isID(e ast.Expr) (string, bool) {
	if id, ok := e.(*ast.ID); ok {
		return id.Name, true
	}
	return "", false
}

/* NOT YET
this should go in the optimizer... when we see a downstream filter at the output
of a join we should push it up before the join, and we should also make sure projection
optimization works through join so that we only need to read the columns that are
used by the join fields and the down stream results.

func liftWhereFilter(aliasID string, where dag.Expr, joins []ast.SQLJoin) *dag.Filter {
	for _, join := range joins {
		// For now, if there are multiple join aliases, be pessimistic
		// and don't try to lift the where.  We can fix this later.
		if _, ok := isID(join.Alias); !ok {
			return nil
		}
	}
	eligible := eligiblePred(aliasID, where)
	if eligible == nil {
		return nil
	}
	return dag.NewFilter(eligible)
}

func eligiblePred(aliasID string, e dag.Expr) dag.Expr {
	switch e := e.(type) {
	case *dag.UnaryExpr:
		if operand := eligiblePred(aliasID, e.Operand); operand != nil {
			return &dag.UnaryExpr{
				Kind:    "UnaryExpr",
				Op:      "!",
				Operand: operand,
			}
		}
	case *dag.Literal:
		return e
	case *dag.Dot:
		// A field reference of the form <aliasID>.x is eligible
		// as the field x because, when lifted it was called x.
		return eligibleFieldRef(aliasID, e)
	case *dag.BinaryExpr:
		lhs := eligiblePred(aliasID, e.LHS)
		rhs := eligiblePred(aliasID, e.RHS)
		if e.Op == "or" {
			if lhs != nil && rhs != nil {
				return &dag.BinaryExpr{
					Kind: "BinaryExpr",
					Op:   e.Op,
					LHS:  lhs,
					RHS:  rhs,
				}
			}
			return nil
		}
		if e.Op == "and" {
			if lhs == nil {
				return rhs
			}
			if rhs == nil {
				return lhs
			}
			return &dag.BinaryExpr{
				Kind: "BinaryExpr",
				Op:   e.Op,
				LHS:  lhs,
				RHS:  rhs,
			}
		}
		if lhs != nil && rhs != nil {
			return &dag.BinaryExpr{
				Kind: "BinaryExpr",
				Op:   e.Op,
				LHS:  lhs,
				RHS:  rhs,
			}
		}
	}
	return nil
}
*/

func eligibleFieldRef(aliasID string, e *dag.Dot) dag.Expr {
	lhs, ok := e.LHS.(*dag.Dot)
	if ok && dag.IsThis(lhs) && lhs.RHS == aliasID {
		return &dag.Dot{
			Kind: "Dot",
			LHS:  &dag.This{Kind: "This"},
			RHS:  e.RHS,
		}
	}
	return nil
}

// XXX this does a join by type and should insteaed just do
// equijoin either with the left/right equality expr or a
// using expr.
func (a *analyzer) semSQLTableRef(e ast.Expr) (dag.Op, error) {
	converted := a.semExpr(e)
	// If an identifier name is given with no definition for that name,
	// then convert it to a type name as it is otherwise expected that
	// the type name will be defined by the data stream.
	if id, ok := dag.TopLevelField(converted); ok {
		e, err := a.scope.LookupExpr(id)
		if err != nil {
			return nil, err
		}
		if e == nil {
			converted = dynamicTypeName(id)
		}
	}
	return dag.NewFilter(&dag.Call{
		Kind: "Call",
		Name: "is",
		Args: []dag.Expr{converted},
	}), nil
}

func (a *analyzer) semSQLAlias(e ast.Expr) (*dag.Cut, string, error) {
	if e == nil {
		return nil, "", nil
	}
	fld := a.semField(e)
	var id string
	if idExpr, ok := e.(*ast.ID); ok {
		id = idExpr.Name
	}
	assignment := dag.Assignment{
		Kind: "Assignment",
		LHS:  fld,
		RHS:  &dag.This{Kind: "This"},
	}
	//XXX cut?! maybe do yield record expr?
	return &dag.Cut{
		Kind: "Cut",
		Args: []dag.Assignment{assignment},
	}, id, nil
}

// For now, each joining table is on the right...
// We don't have logic to not care about the side of the JOIN ON keys...
func (a *analyzer) semSQLJoin(join *ast.SQLJoin, seq dag.Seq) dag.Seq {
	// XXX This is super goofy but for now we require an alias on the
	// right side and combine the entire right side value into the row
	// using the existing join semantics of assignment where the lval
	// lives in the left record and the rval comes from the right.
	alias, ok := findAlias(join.Right)
	if !ok {
		a.error(join, errors.New("SQL joins currently require an table alias on the right lef of the join"))
		seq = append(seq, badOp())
	}
	leftKey, rightKey, err := a.semJoinCond(join.Cond)
	if err != nil {
		//XXX want the join condition position...
		a.error(join, errors.New("SQL joins currently limited to equijoin on fields"))
		return append(seq, badOp())
	}
	leftPath := a.semSQLOp(join.Left, nil)
	rightPath := a.semSQLOp(join.Right, nil)

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
	return append(seq, dagJoin)
}

// XXX I think alias has to be last... not sure we need the recursion
func findAlias(op ast.Op) (string, bool) {
	switch op := op.(type) {
	case *ast.OrderBy:
		return findAlias(op.Op)
	case *ast.Alias:
		return op.Name, true
	}
	return "", false
}

func (a *analyzer) semJoinCond(cond ast.JoinExpr) (*dag.This, *dag.This, error) {
	switch cond := cond.(type) {
	case *ast.JoinOn:
		binary, ok := cond.Expr.(*ast.BinaryExpr)
		if !ok || binary.Op != "==" {
			return nil, nil, errors.New("only equijoins currently supported")
		}
		//XXX we currently require field expressions
		// need to generalize this but that will require work on the
		// runtime join implementation.
		leftKey, ok := a.semField(binary.LHS).(*dag.This)
		if !ok {
			return nil, nil, errors.New("join keys must be field references")
		}
		rightKey, ok := a.semField(binary.RHS).(*dag.This)
		if !ok {
			return nil, nil, errors.New("join keys must be field references")
		}
		return leftKey, rightKey, nil
	case *ast.JoinUsing:
		//XXX
		panic("XXX TBD - JoinUsing")
	default:
		panic("semJoinCond")
	}
}

func sortBy(e dag.Expr) *dag.Sort {
	return sortByMulti([]dag.SortExpr{{Key: e, Order: order.Asc}})
}

func sortByMulti(keys []dag.SortExpr) *dag.Sort {
	return &dag.Sort{
		Kind:    "Sort",
		Args:    keys,
		Reverse: false, //XXX why is this still here? sort -r x:asc, y:desc?!?!
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

func convertSQLSelect(selection sqlSelection) (dag.Op, error) {
	// This is a straight select without a group-by.
	// If all the expressions are aggregators, then we build a group-by.
	// If it's mixed, we return an error.  Otherwise, we do a simple cut.
	var nagg int
	for _, p := range selection {
		if p.agg != nil {
			nagg++
		}
	}
	if nagg == 0 {
		return selection.cut(), nil
	}
	if nagg != len(selection) {
		return nil, errors.New("cannot mix aggregations and non-aggregations without a GROUP BY")
	}
	// Note here that we reconstruct the group-by aggregators instead of
	// using the assignments in ast.SqlExpression.Select since the SQL peg
	// parser does not know whether they are aggregators or function calls,
	// but the sqlPick elements have this determined.  So we take the LHS
	// from the original expression and mix it with the agg that was put
	// in sqlPick.
	var assignments []dag.Assignment
	for _, p := range selection {
		a := dag.Assignment{
			Kind: "Assignment",
			LHS:  p.assignment.LHS,
			RHS:  p.agg,
		}
		assignments = append(assignments, a)
	}
	return &dag.Summarize{
		Kind: "Summarize",
		Aggs: assignments,
	}, nil
}

func (a *analyzer) convertSQLGroupBy(groupByKeys []ast.Expr, selection sqlSelection) (dag.Op, error) {
	var keys field.List
	for _, key := range groupByKeys {
		name := a.sqlField(key)
		//XXX is this the best way to handle nil
		if name != nil {
			keys = append(keys, name)
		}
	}
	// Make sure all group-by keys are in the selection.
	all := selection.fields()
	for _, key := range keys {
		//XXX fix this for select *?
		if !key.In(all) {
			if key.HasPrefixIn(all) {
				return nil, fmt.Errorf("'%s': GROUP BY key cannot be a sub-field of the selected value", key)
			}
			return nil, fmt.Errorf("'%s': GROUP BY key not in selection", key)
		}
	}
	// Make sure all scalars are in the group-by keys.
	scalars := selection.scalars()
	for _, f := range scalars.fields() {
		if !f.In(keys) {
			return nil, fmt.Errorf("'%s': selected expression is missing from GROUP BY clause (and is not an aggregation)", f)
		}
	}
	// Now that the selection and keys have been checked, build the
	// key expressions from the scalars of the select and build the
	// aggregators (aka reducers) from the aggregation functions present
	// in the select clause.
	var keyExprs []dag.Assignment
	for _, p := range scalars {
		keyExprs = append(keyExprs, p.assignment)
	}
	var aggExprs []dag.Assignment
	for _, p := range selection.aggs() {
		aggExprs = append(aggExprs, dag.Assignment{
			Kind: "Assignment",
			LHS:  p.assignment.LHS,
			RHS:  p.agg,
		})
	}
	// XXX how to override limit for spills?
	return &dag.Summarize{
		Kind: "Summarize",
		Keys: keyExprs,
		Aggs: aggExprs,
	}, nil
}

// A sqlPick is one column of a select statement.  We bookkeep here whether
// a column is a scalar expression or an aggregation by looking up the function
// name and seeing if it's an aggregator or not.  We also infer the column
// names so we can do SQL error checking relating the selections to the group-by
// keys, something that is not needed in Zed.
type sqlPick struct {
	name       field.Path
	agg        *dag.Agg
	assignment dag.Assignment
}

type sqlSelection []sqlPick

func (a *analyzer) newSQLSelection(assignments []ast.Assignment) (sqlSelection, error) {
	//XXX update comment
	// Make a cut from a SQL select.  This should just work
	// without having to track identifier names of columns because
	// the transformations will all relable data from stage to stage
	// and Select names refer to the names at the last stage of
	// the table.
	var s sqlSelection
	for _, as := range assignments {
		name, err := a.deriveAs(as)
		if err != nil {
			return nil, err
		}
		agg, err := a.isAgg(as.RHS)
		if err != nil {
			return nil, err
		}
		assignment := a.semAssignment(as)
		s = append(s, sqlPick{name, agg, assignment})
	}
	return s, nil
}

func (s sqlSelection) fields() field.List {
	var fields field.List
	for _, p := range s {
		fields = append(fields, p.name)
	}
	return fields
}

func (s sqlSelection) aggs() sqlSelection {
	var aggs sqlSelection
	for _, p := range s {
		if p.agg != nil {
			aggs = append(aggs, p)
		}
	}
	return aggs
}

func (s sqlSelection) scalars() sqlSelection {
	var scalars sqlSelection
	for _, p := range s {
		if p.agg == nil {
			scalars = append(scalars, p)
		}
	}
	return scalars
}

func (s sqlSelection) cut() *dag.Cut {
	if len(s) == 0 {
		return nil
	}
	var a []dag.Assignment
	for _, p := range s {
		a = append(a, p.assignment)
	}
	return &dag.Cut{
		Kind: "Cut",
		Args: a,
	}
}

func (a *analyzer) isAgg(e ast.Expr) (*dag.Agg, error) {
	call, ok := e.(*ast.Call)
	if !ok {
		//XXX this doesn't work for aggs inside of expressions, sum(x)+sum(y)
		return nil, nil
	}
	if _, err := agg.NewPattern(call.Name.Name, true); err != nil {
		return nil, nil
	}
	var arg ast.Expr
	if len(call.Args) > 1 {
		return nil, fmt.Errorf("%s: wrong number of arguments", call.Name.Name)
	}
	if len(call.Args) == 1 {
		arg = call.Args[0]
	}
	var dagArg dag.Expr
	if arg != nil {
		dagArg = a.semExpr(arg)
	}
	return &dag.Agg{
		Kind: "Agg",
		Name: call.Name.Name,
		Expr: dagArg,
	}, nil
}

func (a *analyzer) deriveAs(as ast.Assignment) (field.Path, error) {
	sa := a.semAssignment(as)
	if this, ok := sa.LHS.(*dag.This); ok {
		return this.Path, nil
	}
	return nil, fmt.Errorf("AS clause not a field")
}

func (a *analyzer) sqlField(e ast.Expr) field.Path {
	if f, ok := a.semField(e).(*dag.This); ok {
		return f.Path
	}
	return nil
}
