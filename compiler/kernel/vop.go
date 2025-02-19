package kernel

import (
	"errors"
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/vam"
	vamexpr "github.com/brimdata/super/runtime/vam/expr"
	vamop "github.com/brimdata/super/runtime/vam/op"
	"github.com/brimdata/super/runtime/vam/op/summarize"
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
		join := vamop.NewJoin(b.rctx.Zctx, anti, inner, leftParent, rightParent, leftKey, rightKey, lhs, rhs)
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
		if p, ok := parent.(interface{ NewConcurrentPuller() vector.Puller }); ok {
			parent = p.NewConcurrentPuller()
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
	case *dag.Cut:
		e, err := b.compileVamAssignmentsToRecordExpression(nil, o.Args)
		if err != nil {
			return nil, err
		}
		return vamop.NewYield(b.zctx(), parent, []vamexpr.Evaluator{e}), nil
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
		dropper := vamexpr.NewDropper(b.zctx(), fields)
		return vamop.NewYield(b.zctx(), parent, []vamexpr.Evaluator{dropper}), nil
	case *dag.FileScan:
		if o.Filter != nil {
			return nil, errors.New("internal error: vector runtime does not support filter pushdown on files")
		}
		return b.env.VectorOpen(b.rctx, b.zctx(), o.Path, o.Format, o.Fields)
	case *dag.Filter:
		e, err := b.compileVamExpr(o.Expr)
		if err != nil {
			return nil, err
		}
		return vamop.NewFilter(b.zctx(), parent, e), nil
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
		initial := []dag.RecordElem{
			&dag.Spread{Kind: "Spread", Expr: &dag.This{Kind: "This"}},
		}
		e, err := b.compileVamAssignmentsToRecordExpression(initial, o.Args)
		if err != nil {
			return nil, err
		}
		return vamop.NewYield(b.zctx(), parent, []vamexpr.Evaluator{vamexpr.NewPutter(b.zctx(), e)}), nil
	case *dag.Rename:
		srcs, dsts, err := b.compileAssignmentsToLvals(o.Args)
		if err != nil {
			return nil, err
		}
		renamer := vamexpr.NewRenamer(b.zctx(), srcs, dsts)
		return vamop.NewYield(b.zctx(), parent, []vamexpr.Evaluator{renamer}), nil
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
	case *dag.Summarize:
		return b.compileVamSummarize(o, parent)
	case *dag.Tail:
		return vamop.NewTail(parent, o.Count), nil
	case *dag.Yield:
		exprs, err := b.compileVamExprs(o.Exprs)
		if err != nil {
			return nil, err
		}
		return vamop.NewYield(b.zctx(), parent, exprs), nil
	default:
		return nil, fmt.Errorf("internal error: unknown dag.Op while compiling for vector runtime: %#v", o)
	}
}

func (b *Builder) compileVamAssignmentsToRecordExpression(initial []dag.RecordElem, assignments []dag.Assignment) (vamexpr.Evaluator, error) {
	elems := initial
	for _, a := range assignments {
		lhs, ok := a.LHS.(*dag.This)
		if !ok {
			return nil, fmt.Errorf("internal error: dynamic field name not supported in vector runtime: %#v", a.LHS)
		}
		elems = append(elems, newDagRecordExprForPath(lhs.Path, a.RHS).Elems...)
	}
	return b.compileVamRecordExpr(&dag.RecordExpr{Kind: "RecordExpr", Elems: elems})
}

func newDagRecordExprForPath(path []string, expr dag.Expr) *dag.RecordExpr {
	if len(path) > 1 {
		expr = newDagRecordExprForPath(path[1:], expr)
	}
	return &dag.RecordExpr{
		Kind: "RecordExpr",
		Elems: []dag.RecordElem{
			&dag.Field{Kind: "Field", Name: path[0], Value: expr},
		},
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
	o := vamop.NewOver(b.zctx(), parent, exprs)
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

func (b *Builder) compileVamSummarize(s *dag.Summarize, parent vector.Puller) (vector.Puller, error) {
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
			return nil, errors.New("invalid lval in groupby key")
		}
		rhs, err := b.compileVamExpr(assignment.RHS)
		if err != nil {
			return nil, err
		}
		keyNames = append(keyNames, lhs.Path)
		keyExprs = append(keyExprs, rhs)
	}
	return summarize.New(parent, b.zctx(), aggNames, aggExprs, aggs, keyNames, keyExprs, s.PartialsIn, s.PartialsOut)
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
	return vamexpr.NewAggregator(name, arg, where)
}
