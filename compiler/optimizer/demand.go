package optimizer

import (
	"slices"

	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/compiler/optimizer/demand"
)

func DemandForSeq(seq dag.Seq, downstreams ...demand.Demand) []demand.Demand {
	for i := len(seq) - 1; i >= 0; i-- {
		downstreams = demandForOp(seq[i], downstreams)
	}
	return downstreams
}

func demandForOp(op dag.Op, downstreams []demand.Demand) []demand.Demand {
	switch op := op.(type) {
	case *dag.Fork:
		var out []demand.Demand
		for i, p := range op.Paths {
			i := min(i, len(downstreams)-1)
			out = append(out, demand.Union(DemandForSeq(p, downstreams[i])...))
		}
		return out
	case *dag.Join:
		downstream := downstreams[0]
		left := demand.GetKey(downstream, op.LeftAlias)
		left = demand.Union(left, demandForExpr(op.LeftKey))
		right := demand.GetKey(downstream, op.RightAlias)
		right = demand.Union(right, demandForExpr(op.RightKey))
		return []demand.Demand{left, right}
	case *dag.Mirror:
		main := DemandForSeq(op.Main, downstreams...)
		mirror := DemandForSeq(op.Mirror, demand.All())
		return []demand.Demand{demand.Union(slices.Concat(main, mirror)...)}
	case *dag.Scatter:
		d := demand.None()
		for i, p := range op.Paths {
			i := min(i, len(downstreams)-1)
			d = demand.Union(d, demand.Union(DemandForSeq(p, downstreams[i])...))
		}
		return []demand.Demand{d}
	case *dag.Scope:
		return DemandForSeq(op.Body, downstreams...)
	case *dag.Switch:
		d := demandForExpr(op.Expr)
		for i, c := range op.Cases {
			d = demand.Union(d, demandForExpr(c.Expr))
			i := min(i, len(downstreams)-1)
			d = demand.Union(d, demand.Union(DemandForSeq(c.Path, downstreams[i])...))
		}
		return []demand.Demand{d}
	case *dag.Vectorize:
		return DemandForSeq(op.Body, downstreams...)
	default:
		return []demand.Demand{demandForSimpleOp(op, demand.Union(downstreams...))}
	}
}

func demandForSimpleOp(op dag.Op, downstream demand.Demand) demand.Demand {
	switch op := op.(type) {
	case *dag.Aggregate:
		d := demand.None()
		for _, assignment := range op.Keys {
			d = demand.Union(d, demandForExpr(assignment.RHS))
		}
		for _, assignment := range op.Aggs {
			d = demand.Union(d, demandForExpr(assignment.RHS))
		}
		return d
	case *dag.Combine:
		return downstream
	case *dag.Cut:
		return demandForAssignments(op.Args, demand.None())
	case *dag.Distinct:
		return demand.Union(downstream, demandForExpr(op.Expr))
	case *dag.Drop:
		return downstream
	case *dag.Explode:
		d := demand.None()
		for _, a := range op.Args {
			d = demand.Union(d, demandForExpr(a))
		}
		return d
	case *dag.Filter:
		return demand.Union(downstream, demandForExpr(op.Expr))
	case *dag.Fuse:
		return demand.All()
	case *dag.Head:
		return downstream
	case *dag.Load:
		return demand.All()
	case *dag.Merge:
		return demandForSortExprs(op.Exprs, downstream)
	case *dag.Output:
		return demand.All()
	case *dag.Pass:
		return downstream
	case *dag.Put:
		return demandForAssignments(op.Args, downstream)
	case *dag.Rename:
		return demandForAssignments(op.Args, downstream)
	case *dag.Shape:
		return downstream
	case *dag.Skip:
		return downstream
	case *dag.Sort:
		return demandForSortExprs(op.Exprs, downstream)
	case *dag.Tail:
		return downstream
	case *dag.Top:
		return demandForSortExprs(op.Exprs, downstream)
	case *dag.Uniq:
		return downstream
	case *dag.Unnest:
		return demandForExpr(op.Expr)
	case *dag.Values:
		d := demand.None()
		for _, e := range op.Exprs {
			d = demand.Union(d, demandForExpr(e))
		}
		return d

	case *dag.CommitMetaScan, *dag.DefaultScan, *dag.Deleter, *dag.DeleteScan, *dag.LakeMetaScan:
		return demand.None()
	case *dag.FileScan:
		if mf := op.Pushdown.MetaFilter; mf != nil {
			mf.Projection = demand.Fields(demandForExpr(mf.Expr))
		}
		d := downstream
		if df := op.Pushdown.DataFilter; df != nil {
			d = demand.Union(d, demandForExpr(df.Expr))
		}
		op.Pushdown.Projection = demand.Fields(d)
		return demand.None()
	case *dag.HTTPScan, *dag.Lister, *dag.NullScan, *dag.PoolMetaScan, *dag.PoolScan:
		return demand.None()
	case *dag.RobotScan:
		return demandForExpr(op.Expr)
	case *dag.SeqScan:
		d := demand.Union(downstream, demandForExpr(op.Filter))
		d = demand.Union(d, demandForExpr(op.KeyPruner))
		op.Fields = demand.Fields(d)
		return demand.None()
	case *dag.Slicer:
		return demand.None()
	}
	panic(op)
}

func demandForExpr(expr dag.Expr) demand.Demand {
	switch expr := expr.(type) {
	case nil:
		return demand.None()
	case *dag.Agg:
		return demand.Union(demandForExpr(expr.Expr), demandForExpr(expr.Where))
	case *dag.ArrayExpr:
		return demandForArrayOrSetExpr(expr.Elems)
	case *dag.BinaryExpr:
		return demand.Union(demandForExpr(expr.LHS), demandForExpr(expr.RHS))
	case *dag.Call:
		d := demand.None()
		if expr.Name == "every" {
			d = demand.Key("ts", demand.All())
		}
		for _, a := range expr.Args {
			d = demand.Union(d, demandForExpr(a))
		}
		return d
	case *dag.Conditional:
		return demand.Union(demandForExpr(expr.Cond),
			demand.Union(demandForExpr(expr.Then), demandForExpr(expr.Else)))
	case *dag.Dot:
		return demandForExpr(expr.LHS)
	case *dag.Func:
		// return demand.All()
	case *dag.IndexExpr:
		return demand.Union(demandForExpr(expr.Expr), demandForExpr(expr.Index))
	case *dag.IsNullExpr:
		return demandForExpr(expr.Expr)
	case *dag.Literal:
		return demand.None()
	case *dag.MapCall:
		return demandForExpr(expr.Expr)
	case *dag.MapExpr:
		d := demand.None()
		for _, e := range expr.Entries {
			d = demand.Union(d, demandForExpr(e.Key))
			d = demand.Union(d, demandForExpr(e.Value))
		}
		return d
	case *dag.QueryExpr:
		return demand.Union(DemandForSeq(expr.Body, demand.All())...)
	case *dag.RecordExpr:
		d := demand.None()
		for _, e := range expr.Elems {
			switch e := e.(type) {
			case *dag.Field:
				d = demand.Union(d, demandForExpr(e.Value))
			case *dag.Spread:
				d = demand.Union(d, demandForExpr(e.Expr))
			default:
				panic(e)
			}
		}
		return d
	case *dag.RegexpMatch:
		return demandForExpr(expr.Expr)
	case *dag.RegexpSearch:
		return demandForExpr(expr.Expr)
	case *dag.Search:
		return demandForExpr(expr.Expr)
	case *dag.SetExpr:
		return demandForArrayOrSetExpr(expr.Elems)
	case *dag.SliceExpr:
		return demand.Union(demandForExpr(expr.Expr),
			demand.Union(demandForExpr(expr.From), demandForExpr(expr.To)))
	case *dag.This:
		d := demand.All()
		for i := len(expr.Path) - 1; i >= 0; i-- {
			d = demand.Key(expr.Path[i], d)
		}
		return d
	case *dag.UnaryExpr:
		return demandForExpr(expr.Operand)
	case *dag.UnnestExpr:
		return demandForExpr(expr.Expr)
	}
	panic(expr)
}

func demandForArrayOrSetExpr(elems []dag.VectorElem) demand.Demand {
	d := demand.None()
	for _, e := range elems {
		switch e := e.(type) {
		case *dag.Spread:
			d = demand.Union(d, demandForExpr(e.Expr))
		case *dag.VectorValue:
			d = demand.Union(d, demandForExpr(e.Expr))
		default:
			panic(e)
		}
	}
	return d
}

func demandForAssignments(assignments []dag.Assignment, downstream demand.Demand) demand.Demand {
	d := downstream
	for _, a := range assignments {
		if _, ok := a.LHS.(*dag.This); ok {
			// Assignment clobbers a static field.
			d = demand.Delete(d, demandForExpr(a.LHS))
		} else {
			// Add anything needed by a dynamic field.
			d = demand.Union(d, demandForExpr(a.LHS))
		}
		d = demand.Union(d, demandForExpr(a.RHS))
	}
	return d
}

func demandForSortExprs(sortExprs []dag.SortExpr, downstream demand.Demand) demand.Demand {
	if len(sortExprs) == 0 {
		// Need all fields to guess sort key.
		return demand.All()
	}
	d := downstream
	for _, s := range sortExprs {
		d = demand.Union(d, demandForExpr(s.Key))
	}
	return d
}
