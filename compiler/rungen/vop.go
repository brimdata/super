package rungen

import (
	"errors"
	"fmt"
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/op/fuse"
	"github.com/brimdata/super/runtime/sam/op/infer"
	vamexpr "github.com/brimdata/super/runtime/vam/expr"
	vamop "github.com/brimdata/super/runtime/vam/op"
	"github.com/brimdata/super/runtime/vam/op/aggregate"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/vector/vio"
)

// compile compiles a DAG into a graph of runtime operators, and returns
// the leaves.
func (b *Builder) compileVam(o dag.Op, parents []vio.Puller) ([]vio.Puller, error) {
	switch o := o.(type) {
	case *dag.CombineOp:
		return []vio.Puller{b.combineVam(parents)}, nil
	case *dag.ForkOp:
		return b.compileVamFork(o, b.combineVam(parents))
	case *dag.HashJoinOp:
		if len(parents) != 2 {
			return nil, ErrJoinParents
		}
		leftKey, err := b.compileVamExpr(o.LeftKey)
		if err != nil {
			return nil, err
		}
		rightKey, err := b.compileVamExpr(o.RightKey)
		if err != nil {
			return nil, err
		}
		join := vamop.NewHashJoin(b.rctx, o.Style, parents[0], parents[1], leftKey, rightKey, o.LeftAlias, o.RightAlias)
		return []vio.Puller{join}, nil
	case *dag.JoinOp:
		if len(parents) != 2 {
			return nil, ErrJoinParents
		}
		var cond vamexpr.Evaluator
		if o.Cond != nil {
			var err error
			cond, err = b.compileVamExpr(o.Cond)
			if err != nil {
				return nil, err
			}
		}
		join := vamop.NewNestedLoopJoin(b.rctx, parents[0], parents[1], o.Style, o.LeftAlias, o.RightAlias, cond)
		return []vio.Puller{join}, nil
	case *dag.MergeOp:
		exprs, err := b.compileSortExprs(o.Exprs)
		if err != nil {
			return nil, err
		}
		cmp := expr.NewComparator(exprs...).WithMissingAsNull()
		return []vio.Puller{vamop.NewMerge(b.rctx, parents, cmp.Compare)}, nil
	case *dag.ScatterOp:
		return b.compileVamScatter(o, parents)
	case *dag.SwitchOp:
		parent := b.combineVam(parents)
		if o.Expr != nil {
			return b.compileVamExprSwitch(o, parent)
		}
		return b.compileVamSwitch(o, parent)
	default:
		p, err := b.compileVamLeaf(o, b.combineVam(parents))
		if err != nil {
			return nil, err
		}
		return []vio.Puller{p}, nil
	}
}

func (b *Builder) combineVam(pullers []vio.Puller) vio.Puller {
	switch len(pullers) {
	case 0:
		return nil
	case 1:
		return pullers[0]
	}
	return vamop.NewCombine(b.rctx, pullers)
}

func (b *Builder) compileVamFork(fork *dag.ForkOp, parent vio.Puller) ([]vio.Puller, error) {
	var f *vamop.Fork
	if parent != nil {
		f = vamop.NewFork(b.rctx, parent)
	}
	var exits []vio.Puller
	for _, seq := range fork.Paths {
		var parent vio.Puller
		if f != nil && !isEntry(seq) {
			parent = f.AddBranch()
		}
		exit, err := b.compileVamSeq(seq, []vio.Puller{parent})
		if err != nil {
			return nil, err
		}
		exits = append(exits, exit...)
	}
	return exits, nil
}

func (b *Builder) compileVamScatter(scatter *dag.ScatterOp, parents []vio.Puller) ([]vio.Puller, error) {
	if len(parents) != 1 {
		return nil, errors.New("internal error: scatter operator requires a single parent")
	}
	var concurrentPullers []vio.Puller
	if f, ok := parents[0].(*vamop.FileScan); ok {
		concurrentPullers = f.NewConcurrentPullers(len(scatter.Paths))
	}
	var ops []vio.Puller
	for i, seq := range scatter.Paths {
		parent := parents[0]
		if len(concurrentPullers) > 0 {
			parent = concurrentPullers[i]
		}
		op, err := b.compileVamSeq(seq, []vio.Puller{parent})
		if err != nil {
			return nil, err
		}
		ops = append(ops, op...)
	}
	return ops, nil
}

func (b *Builder) compileVamExprSwitch(swtch *dag.SwitchOp, parent vio.Puller) ([]vio.Puller, error) {
	e, err := b.compileVamExpr(swtch.Expr)
	if err != nil {
		return nil, err
	}
	s := vamop.NewExprSwitch(b.rctx, parent, e)
	var exits []vio.Puller
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
		parents, err := b.compileVamSeq(c.Path, []vio.Puller{s.AddCase(val)})
		if err != nil {
			return nil, err
		}
		exits = append(exits, parents...)
	}
	return exits, nil
}

func (b *Builder) compileVamSwitch(swtch *dag.SwitchOp, parent vio.Puller) ([]vio.Puller, error) {
	s := vamop.NewSwitch(b.rctx, parent)
	var exits []vio.Puller
	for _, c := range swtch.Cases {
		e, err := b.compileVamExpr(c.Expr)
		if err != nil {
			return nil, fmt.Errorf("compiling switch case filter: %w", err)
		}
		exit, err := b.compileVamSeq(c.Path, []vio.Puller{s.AddCase(e)})
		if err != nil {
			return nil, err
		}
		exits = append(exits, exit...)
	}
	return exits, nil
}

func (b *Builder) compileVamMain(main *dag.Main, parents []vio.Puller) ([]vio.Puller, error) {
	for _, f := range main.Funcs {
		b.funcs[f.Tag] = f
	}
	return b.compileVamSeq(main.Body, parents)
}

func (b *Builder) compileVamLeaf(o dag.Op, parent vio.Puller) (vio.Puller, error) {
	switch o := o.(type) {
	case *dag.AggregateOp:
		return b.compileVamAggregate(o, parent)
	case *dag.CountOp:
		var e vamexpr.Evaluator
		if o.Expr != nil {
			var err error
			if e, err = b.compileVamExpr(o.Expr); err != nil {
				return nil, err
			}
		}
		return vamop.NewCount(b.rctx.Sctx, parent, o.Alias, e), nil
	case *dag.CutOp:
		rec, err := newRecordExprFromAssignments(o.Args)
		if err != nil {
			return nil, err
		}
		e, err := b.compileVamRecordExpr(rec)
		if err != nil {
			return nil, err
		}
		e = vamexpr.NewDequiet(b.sctx(), e)
		return vamop.NewValues(b.sctx(), parent, []vamexpr.Evaluator{e}), nil
	case *dag.DebugOp:
		e, err := b.compileVamExpr(o.Expr)
		if err != nil {
			return nil, err
		}
		filter, err := b.compileVamExprWithEmpty(o.Filter)
		if err != nil {
			return nil, err
		}
		d := vamop.NewDebug(b.rctx, e, filter, b.debugs, parent)
		return d, nil
	case *dag.DefaultScan:
		sbufPuller, err := b.compileLeaf(o, nil)
		if err != nil {
			return nil, err
		}
		return sbuf.NewDematerializer(b.sctx(), sbufPuller), nil
	case *dag.DistinctOp:
		e, err := b.compileVamExpr(o.Expr)
		if err != nil {
			return nil, err
		}
		return vamop.NewDistinct(parent, e), nil
	case *dag.DropOp:
		fields := make(field.List, 0, len(o.Args))
		for _, e := range o.Args {
			fields = append(fields, e.(*dag.ThisExpr).Path)
		}
		dropper := vamexpr.NewDropper(b.sctx(), fields)
		return vamop.NewValues(b.sctx(), parent, []vamexpr.Evaluator{dropper}), nil
	case *dag.FileScan:
		var metaProjection []field.Path
		var metaFilter dag.Expr
		if mf := o.Pushdown.MetaFilter; mf != nil {
			metaFilter = mf.Expr
			metaProjection = mf.Projection
		}
		pushdown := b.newMetaPushdown(metaFilter, o.Pushdown.Projection, metaProjection, o.Pushdown.Unordered)
		return vamop.NewFileScan(b.rctx, b.env, o.Paths, o.Format, pushdown), nil
	case *dag.FilterOp:
		e, err := b.compileVamExpr(o.Expr)
		if err != nil {
			return nil, err
		}
		return vamop.NewFilter(b.sctx(), parent, e), nil
	case *dag.FuseOp:
		return sbuf.NewDematerializer(b.sctx(), fuse.New(b.rctx, o.Complete, sbuf.NewMaterializer(parent))), nil
	case *dag.HeadOp:
		return vamop.NewHead(parent, o.Count), nil
	case *dag.InferOp:
		return sbuf.NewDematerializer(b.sctx(), infer.New(b.rctx, sbuf.NewMaterializer(parent), o.Limit)), nil
	case *dag.NullScan:
		return sbuf.NewDematerializer(b.sctx(), sbuf.NewPuller(sbuf.NewArray([]super.Value{super.Null}))), nil
	case *dag.OutputOp:
		b.channels[o.Name] = append(b.channels[o.Name], parent)
		return parent, nil
	case *dag.PassOp:
		return parent, nil
	case *dag.PutOp:
		rec, err := newRecordExprFromAssignments(o.Args)
		if err != nil {
			return nil, err
		}
		mergeRecordExprWithPath(rec, nil)
		e, err := b.compileVamRecordExpr(rec)
		if err != nil {
			return nil, err
		}
		e = vamexpr.NewDequiet(b.sctx(), e)
		putter := vamexpr.NewPutter(b.sctx(), e)
		return vamop.NewValues(b.sctx(), parent, []vamexpr.Evaluator{putter}), nil
	case *dag.RenameOp:
		srcs, dsts, err := b.compileAssignmentsToLvals(o.Args)
		if err != nil {
			return nil, err
		}
		renamer := vamexpr.NewRenamer(b.sctx(), srcs, dsts)
		return vamop.NewValues(b.sctx(), parent, []vamexpr.Evaluator{renamer}), nil
	case *dag.RobotScan:
		e, err := b.compileVamExpr(o.Expr)
		if err != nil {
			return nil, err
		}
		return vamop.NewRobot(b.rctx, b.env, parent, e, o.Format, b.newPushdown(o.Filter, nil)), nil
	case *dag.SkipOp:
		return vamop.NewSkip(parent, o.Count), nil
	case *dag.TopOp:
		sbufPuller, err := b.compileLeaf(o, sbuf.NewMaterializer(parent))
		if err != nil {
			return nil, err
		}
		return sbuf.NewDematerializer(b.sctx(), sbufPuller), nil
	case *dag.SortOp:
		var sortExprs []expr.SortExpr
		for _, e := range o.Exprs {
			k, err := b.compileExpr(e.Key)
			if err != nil {
				return nil, err
			}
			sortExprs = append(sortExprs, expr.NewSortExpr(k, e.Order, e.Nulls))
		}
		return vamop.NewSort(b.rctx, parent, sortExprs, o.Reverse), nil
	case *dag.TailOp:
		return vamop.NewTail(parent, o.Count), nil
	case *dag.UniqOp:
		sbufPuller, err := b.compileLeaf(o, sbuf.NewMaterializer(parent))
		if err != nil {
			return nil, err
		}
		return sbuf.NewDematerializer(b.sctx(), sbufPuller), nil
	case *dag.UnnestOp:
		e, err := b.compileVamExpr(o.Expr)
		if err != nil {
			return nil, err
		}
		return vamop.NewUnnest(b.sctx(), parent, e), nil
	case *dag.ValuesOp:
		exprs, err := b.compileVamExprs(o.Exprs)
		if err != nil {
			return nil, err
		}
		return vamop.NewValues(b.sctx(), parent, exprs), nil
	default:
		return nil, fmt.Errorf("internal error: unknown dag.Op while compiling for vector runtime: %#v", o)
	}
}

func newRecordExprFromAssignments(assignments []dag.Assignment) (*dag.RecordExpr, error) {
	rec := &dag.RecordExpr{Kind: "RecordExpr"}
	for _, a := range assignments {
		lhs, ok := a.LHS.(*dag.ThisExpr)
		if !ok {
			return nil, fmt.Errorf("internal error: dynamic field name not supported: %#v", a.LHS)
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
	spread := &dag.Spread{Kind: "Spread", Expr: dag.NewThis(path)}
	rec.Elems = append([]dag.RecordElem{spread}, rec.Elems...)
	for _, elem := range rec.Elems {
		if field, ok := elem.(*dag.Field); ok {
			if childrec, ok := field.Value.(*dag.RecordExpr); ok {
				mergeRecordExprWithPath(childrec, append(path, field.Name))
			}
		}
	}
}

func (b *Builder) compileVamSeq(seq dag.Seq, parents []vio.Puller) ([]vio.Puller, error) {
	for _, o := range seq {
		var err error
		parents, err = b.compileVam(o, parents)
		if err != nil {
			return nil, err
		}
	}
	return parents, nil
}

func (b *Builder) compileVamAggregate(s *dag.AggregateOp, parent vio.Puller) (vio.Puller, error) {
	// compile aggs
	var aggNames []field.Path
	var aggExprs []vamexpr.Evaluator
	var aggs []*vamexpr.Aggregator
	for _, assignment := range s.Aggs {
		aggNames = append(aggNames, assignment.LHS.(*dag.ThisExpr).Path)
		lhs, err := b.compileVamExpr(assignment.LHS)
		if err != nil {
			return nil, err
		}
		aggExprs = append(aggExprs, lhs)
		agg, err := b.compileVamAgg(assignment.RHS.(*dag.AggExpr))
		if err != nil {
			return nil, err
		}
		aggs = append(aggs, agg)
	}
	// compile keys
	var keyNames []field.Path
	var keyExprs []vamexpr.Evaluator
	for _, assignment := range s.Keys {
		lhs, ok := assignment.LHS.(*dag.ThisExpr)
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
	if len(keyExprs) == 0 {
		return aggregate.NewScalar(parent, b.sctx(), aggs, aggNames, aggExprs, s.PartialsIn, s.PartialsOut)
	}
	return aggregate.New(parent, b.sctx(), aggNames, aggExprs, aggs, keyNames, keyExprs, s.PartialsIn, s.PartialsOut)
}

func (b *Builder) compileVamAgg(agg *dag.AggExpr) (*vamexpr.Aggregator, error) {
	name := agg.Name
	var err error
	var arg vamexpr.Evaluator
	if agg.Expr != nil {
		arg, err = b.compileVamExpr(agg.Expr)
		if err != nil {
			return nil, err
		}
	}
	var filter vamexpr.Evaluator
	if agg.Filter != nil {
		filter, err = b.compileVamExpr(agg.Filter)
		if err != nil {
			return nil, err
		}
	}
	return vamexpr.NewAggregator(name, agg.Distinct, arg, filter)
}
