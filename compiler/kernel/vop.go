package kernel

import (
	"errors"
	"fmt"
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/vam"
	vamexpr "github.com/brimdata/super/runtime/vam/expr"
	vamop "github.com/brimdata/super/runtime/vam/op"
	"github.com/brimdata/super/runtime/vam/op/aggregate"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zbuf"
)

// compile compiles a DAG into a graph of runtime operators, and returns
// the leaves.
func (b *Builder) compileVam(o dag.Op, parents []vector.Puller) ([]vector.Puller, error) {
	switch o := o.(type) {
	case *dag.Combine:
		return []vector.Puller{vamop.NewCombine(b.rctx, parents)}, nil
	case *dag.Fork:
		return b.compileVamFork(o, parents)
	case *dag.Join:
		if len(parents) != 2 {
			return nil, ErrJoinParents
		}
		assignments, err := b.compileAssignments(o.Args)
		if err != nil {
			return nil, err
		}
		lhs, rhs := splitAssignments(assignments)
		leftKey, err := b.compileVamExpr(o.LeftKey)
		if err != nil {
			return nil, err
		}
		rightKey, err := b.compileVamExpr(o.RightKey)
		if err != nil {
			return nil, err
		}
		leftParent, rightParent := parents[0], parents[1]
		var anti, inner bool
		switch o.Style {
		case "anti":
			anti = true
		case "inner":
			inner = true
		case "left":
		case "right":
			leftKey, rightKey = rightKey, leftKey
			leftParent, rightParent = rightParent, leftParent
		default:
			return nil, fmt.Errorf("unknown kind of join: '%s'", o.Style)
		}
		join := vamop.NewJoin(b.rctx.Sctx, anti, inner, leftParent, rightParent, leftKey, rightKey, lhs, rhs)
		return []vector.Puller{join}, nil
	case *dag.Merge:
		b.resetResetters()
		e, err := b.compileExpr(o.Expr)
		if err != nil {
			return nil, err
		}
		cmp := expr.NewComparator(true, expr.NewSortEvaluator(e, o.Order)).WithMissingAsNull()
		return []vector.Puller{vamop.NewMerge(b.rctx, parents, cmp.Compare)}, nil
	case *dag.Scatter:
		return b.compileVamScatter(o, parents)
	case *dag.Scope:
		return b.compileVamScope(o, parents)
	case *dag.Switch:
		if o.Expr != nil {
			return b.compileVamExprSwitch(o, parents)
		}
		return b.compileVamSwitch(o, parents)
	default:
		var parent vector.Puller
		if len(parents) == 1 {
			parent = parents[0]
		} else if len(parents) > 1 {
			parent = vamop.NewCombine(b.rctx, parents)
		}
		p, err := b.compileVamLeaf(o, parent)
		if err != nil {
			return nil, err
		}
		return []vector.Puller{p}, nil
	}
}

func (b *Builder) compileVamScan(scan *dag.SeqScan, parent zbuf.Puller) (vector.Puller, error) {
	pool, err := b.lookupPool(scan.Pool)
	if err != nil {
		return nil, err
	}
	//XXX check VectorCache not nil
	return vamop.NewScanner(b.rctx, b.env.Lake().VectorCache(), parent, pool, scan.Fields, nil, nil), nil
}

func (b *Builder) compileVamFork(fork *dag.Fork, parents []vector.Puller) ([]vector.Puller, error) {
	var f *vamop.Fork
	switch len(parents) {
	case 0:
		// No parents: no need for a fork since every op gets a nil parent.
	case 1:
		// Single parent: insert a fork for n-way fanout.
		f = vamop.NewFork(b.rctx, parents[0])
	default:
		// Multiple parents: insert a combine followed by a fork for n-way fanout.
		f = vamop.NewFork(b.rctx, vamop.NewCombine(b.rctx, parents))
	}
	var exits []vector.Puller
	for _, seq := range fork.Paths {
		var parent vector.Puller
		if f != nil && !isEntry(seq) {
			parent = f.AddBranch()
		}
		exit, err := b.compileVamSeq(seq, []vector.Puller{parent})
		if err != nil {
			return nil, err
		}
		exits = append(exits, exit...)
	}
	return exits, nil
}

func (b *Builder) compileVamScatter(scatter *dag.Scatter, parents []vector.Puller) ([]vector.Puller, error) {
	if len(parents) != 1 {
		return nil, errors.New("internal error: scatter operator requires a single parent")
	}
	var ops []vector.Puller
	for _, seq := range scatter.Paths {
		parent := parents[0]
		if p, ok := parent.(interface{ NewConcurrentPuller() (vector.Puller, error) }); ok {
			p, err := p.NewConcurrentPuller()
			if err != nil {
				return nil, err
			}
			parent = p
		}
		op, err := b.compileVamSeq(seq, []vector.Puller{parent})
		if err != nil {
			return nil, err
		}
		ops = append(ops, op...)
	}
	return ops, nil
}

func (b *Builder) compileVamExprSwitch(swtch *dag.Switch, parents []vector.Puller) ([]vector.Puller, error) {
	parent := parents[0]
	if len(parents) > 1 {
		parent = vamop.NewCombine(b.rctx, parents)
	}
	e, err := b.compileVamExpr(swtch.Expr)
	if err != nil {
		return nil, err
	}
	s := vamop.NewExprSwitch(b.rctx, parent, e)
	var exits []vector.Puller
	for _, c := range swtch.Cases {
		var val *super.Value
		if c.Expr != nil {
			val2, err := b.evalAtCompileTime(c.Expr)
			if err != nil {
				return nil, err
			}
			if val2.IsError() {
				return nil, errors.New("switch case is not a constant expression")
			}
			val = &val2
		}
		parents, err := b.compileVamSeq(c.Path, []vector.Puller{s.AddCase(val)})
		if err != nil {
			return nil, err
		}
		exits = append(exits, parents...)
	}
	return exits, nil
}

func (b *Builder) compileVamSwitch(swtch *dag.Switch, parents []vector.Puller) ([]vector.Puller, error) {
	parent := parents[0]
	if len(parents) > 1 {
		parent = vamop.NewCombine(b.rctx, parents)
	}
	s := vamop.NewSwitch(b.rctx, parent)
	var exits []vector.Puller
	for _, c := range swtch.Cases {
		e, err := b.compileVamExpr(c.Expr)
		if err != nil {
			return nil, fmt.Errorf("compiling switch case filter: %w", err)
		}
		exit, err := b.compileVamSeq(c.Path, []vector.Puller{s.AddCase(e)})
		if err != nil {
			return nil, err
		}
		exits = append(exits, exit...)
	}
	return exits, nil
}

func (b *Builder) compileVamScope(scope *dag.Scope, parents []vector.Puller) ([]vector.Puller, error) {
	if len(scope.Funcs) > 0 {
		return nil, errors.New("udfs not currently supported in vector runtime")
	}
	return b.compileVamSeq(scope.Body, parents)
}

func (b *Builder) compileVamLeaf(o dag.Op, parent vector.Puller) (vector.Puller, error) {
	switch o := o.(type) {
	case *dag.Aggregate:
		return b.compileVamAggregate(o, parent)
	case *dag.Cut:
		rec, err := vamNewRecordExprFromAssignments(o.Args)
		if err != nil {
			return nil, err
		}
		e, err := b.compileVamRecordExpr(rec)
		if err != nil {
			return nil, err
		}
		return vamop.NewYield(b.sctx(), parent, []vamexpr.Evaluator{e}), nil
	case *dag.DefaultScan:
		zbufPuller, err := b.compileLeaf(o, nil)
		if err != nil {
			return nil, err
		}
		return vam.NewDematerializer(zbufPuller), nil
	case *dag.Distinct:
		e, err := b.compileVamExpr(o.Expr)
		if err != nil {
			return nil, err
		}
		return vamop.NewDistinct(parent, e), nil
	case *dag.Drop:
		fields := make(field.List, 0, len(o.Args))
		for _, e := range o.Args {
			fields = append(fields, e.(*dag.This).Path)
		}
		dropper := vamexpr.NewDropper(b.sctx(), fields)
		return vamop.NewYield(b.sctx(), parent, []vamexpr.Evaluator{dropper}), nil
	case *dag.FileScan:
		var metaProjection []field.Path
		var metaFilter dag.Expr
		if mf := o.Pushdown.MetaFilter; mf != nil {
			metaFilter = mf.Expr
			metaProjection = mf.Projection
		}
		pushdown := b.newMetaPushdown(metaFilter, o.Pushdown.Projection, metaProjection)
		return b.env.VectorOpen(b.rctx, b.sctx(), o.Path, o.Format, pushdown)
	case *dag.Filter:
		e, err := b.compileVamExpr(o.Expr)
		if err != nil {
			return nil, err
		}
		return vamop.NewFilter(b.sctx(), parent, e), nil
	case *dag.Head:
		return vamop.NewHead(parent, o.Count), nil
	case *dag.NullScan:
		return vam.NewDematerializer(zbuf.NewPuller(zbuf.NewArray([]super.Value{super.Null}))), nil

	case *dag.Output:
		b.channels[o.Name] = append(b.channels[o.Name], vam.NewMaterializer(parent))
		return parent, nil
	case *dag.Over:
		return b.compileVamOver(o, parent)
	case *dag.Pass:
		return parent, nil
	case *dag.Put:
		rec, err := vamNewRecordExprFromAssignments(o.Args)
		if err != nil {
			return nil, err
		}
		mergeRecordExprWithPath(rec, nil)
		e, err := b.compileVamRecordExpr(rec)
		if err != nil {
			return nil, err
		}
		return vamop.NewYield(b.sctx(), parent, []vamexpr.Evaluator{vamexpr.NewPutter(b.sctx(), e)}), nil
	case *dag.Rename:
		srcs, dsts, err := b.compileAssignmentsToLvals(o.Args)
		if err != nil {
			return nil, err
		}
		renamer := vamexpr.NewRenamer(b.sctx(), srcs, dsts)
		return vamop.NewYield(b.sctx(), parent, []vamexpr.Evaluator{renamer}), nil
	case *dag.Top:
		zbufPuller, err := b.compileLeaf(o, vam.NewMaterializer(parent))
		if err != nil {
			return nil, err
		}
		return vam.NewDematerializer(zbufPuller), nil
	case *dag.Sort:
		b.resetResetters()
		var sortExprs []expr.SortEvaluator
		for _, s := range o.Args {
			k, err := b.compileExpr(s.Key)
			if err != nil {
				return nil, err
			}
			sortExprs = append(sortExprs, expr.NewSortEvaluator(k, s.Order))
		}
		return vamop.NewSort(b.rctx, parent, sortExprs, o.NullsFirst, o.Reverse, b.resetters), nil
	case *dag.Tail:
		return vamop.NewTail(parent, o.Count), nil
	case *dag.Uniq:
		zbufPuller, err := b.compileLeaf(o, vam.NewMaterializer(parent))
		if err != nil {
			return nil, err
		}
		return vam.NewDematerializer(zbufPuller), nil
	case *dag.Yield:
		exprs, err := b.compileVamExprs(o.Exprs)
		if err != nil {
			return nil, err
		}
		return vamop.NewYield(b.sctx(), parent, exprs), nil
	default:
		return nil, fmt.Errorf("internal error: unknown dag.Op while compiling for vector runtime: %#v", o)
	}
}

func vamNewRecordExprFromAssignments(assignments []dag.Assignment) (*dag.RecordExpr, error) {
	rec := &dag.RecordExpr{Kind: "RecordExpr"}
	for _, a := range assignments {
		lhs, ok := a.LHS.(*dag.This)
		if !ok {
			return nil, fmt.Errorf("internal error: dynamic field name not supported in vector runtime: %#v", a.LHS)
		}
		addPathToRecordExpr(rec, lhs.Path, a.RHS)
	}
	return rec, nil
}

func addPathToRecordExpr(rec *dag.RecordExpr, path []string, expr dag.Expr) {
	if len(path) == 1 {
		rec.Elems = append(rec.Elems, &dag.Field{Kind: "Field", Name: path[0], Value: expr})
		return
	}
	i := slices.IndexFunc(rec.Elems, func(elem dag.RecordElem) bool {
		f, ok := elem.(*dag.Field)
		return ok && f.Name == path[0]
	})
	if i == -1 {
		i = len(rec.Elems)
		rec.Elems = append(rec.Elems, &dag.Field{Kind: "Field", Name: path[0], Value: &dag.RecordExpr{Kind: "RecordExpr"}})
	}
	addPathToRecordExpr(rec.Elems[i].(*dag.Field).Value.(*dag.RecordExpr), path[1:], expr)
}

func mergeRecordExprWithPath(rec *dag.RecordExpr, path []string) {
	spread := &dag.Spread{Kind: "Spread", Expr: &dag.This{Kind: "This", Path: path}}
	rec.Elems = append([]dag.RecordElem{spread}, rec.Elems...)
	for _, elem := range rec.Elems {
		if field, ok := elem.(*dag.Field); ok {
			if childrec, ok := field.Value.(*dag.RecordExpr); ok {
				mergeRecordExprWithPath(childrec, append(path, field.Name))
			}
		}
	}
}

func (b *Builder) compileVamOver(over *dag.Over, parent vector.Puller) (vector.Puller, error) {
	// withNames, withExprs, err := b.compileDefs(over.Defs)
	// if err != nil {
	// 	return nil, err
	// }
	exprs, err := b.compileVamExprs(over.Exprs)
	if err != nil {
		return nil, err
	}
	o := vamop.NewOver(b.sctx(), parent, exprs)
	if over.Body == nil {
		return o, nil
	}
	scope := o.NewScope()
	exits, err := b.compileVamSeq(over.Body, []vector.Puller{scope})
	if err != nil {
		return nil, err
	}
	var exit vector.Puller
	if len(exits) == 1 {
		exit = exits[0]
	} else {
		// This can happen when output of over body
		// is a fork or switch.
		exit = vamop.NewCombine(b.rctx, exits)
	}
	return o.NewScopeExit(exit), nil
}

func (b *Builder) compileVamSeq(seq dag.Seq, parents []vector.Puller) ([]vector.Puller, error) {
	for _, o := range seq {
		var err error
		parents, err = b.compileVam(o, parents)
		if err != nil {
			return nil, err
		}
	}
	return parents, nil
}

func (b *Builder) compileVamAggregate(s *dag.Aggregate, parent vector.Puller) (vector.Puller, error) {
	// compile aggs
	var aggNames []field.Path
	var aggExprs []vamexpr.Evaluator
	var aggs []*vamexpr.Aggregator
	for _, assignment := range s.Aggs {
		aggNames = append(aggNames, assignment.LHS.(*dag.This).Path)
		lhs, err := b.compileVamExpr(assignment.LHS)
		if err != nil {
			return nil, err
		}
		aggExprs = append(aggExprs, lhs)
		agg, err := b.compileVamAgg(assignment.RHS.(*dag.Agg))
		if err != nil {
			return nil, err
		}
		aggs = append(aggs, agg)
	}
	// compile keys
	var keyNames []field.Path
	var keyExprs []vamexpr.Evaluator
	for _, assignment := range s.Keys {
		lhs, ok := assignment.LHS.(*dag.This)
		if !ok {
			return nil, errors.New("invalid lval in grouping key")
		}
		rhs, err := b.compileVamExpr(assignment.RHS)
		if err != nil {
			return nil, err
		}
		keyNames = append(keyNames, lhs.Path)
		keyExprs = append(keyExprs, rhs)
	}
	return aggregate.New(parent, b.sctx(), aggNames, aggExprs, aggs, keyNames, keyExprs, s.PartialsIn, s.PartialsOut)
}

func (b *Builder) compileVamAgg(agg *dag.Agg) (*vamexpr.Aggregator, error) {
	name := agg.Name
	var err error
	var arg vamexpr.Evaluator
	if agg.Expr != nil {
		arg, err = b.compileVamExpr(agg.Expr)
		if err != nil {
			return nil, err
		}
	}
	var where vamexpr.Evaluator
	if agg.Where != nil {
		where, err = b.compileVamExpr(agg.Where)
		if err != nil {
			return nil, err
		}
	}
	return vamexpr.NewAggregator(name, agg.Distinct, arg, where)
}
