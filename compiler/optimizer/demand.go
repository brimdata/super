package optimizer

import (
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/compiler/optimizer/demand"
)

func DemandForSeq(seq dag.Seq, downstream demand.Demand) demand.Demand {
	for i := len(seq) - 1; i >= 0; i-- {
		downstream = demandForOp(seq[i], downstream)
	}
	return downstream
}

func demandForOp(op dag.Op, downstream demand.Demand) demand.Demand {
	switch op := op.(type) {
	case *dag.Combine:
		return downstream
	case *dag.Cut:
		return demandForAssignments(op.Args, demand.None())
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
	case *dag.Fork:
		d := demand.None()
		for _, p := range op.Paths {
			d = demand.Union(d, DemandForSeq(p, downstream))
		}
		return d
	case *dag.Fuse:
		return demand.All()
	case *dag.Head:
		return downstream
	case *dag.Join:
		d := downstream
		d = demand.Union(d, demandForExpr(op.LeftKey))
		d = demand.Union(d, demandForExpr(op.RightKey))
		return demandForAssignments(op.Args, d)
	case *dag.Load:
		return demand.All()
	case *dag.Merge:
		return demand.Union(downstream, demandForExpr(op.Expr))
	case *dag.Mirror:
		return demand.Union(DemandForSeq(op.Main, demand.All()),
			DemandForSeq(op.Mirror, demand.All()))
	case *dag.Output:
		return demand.All()
	case *dag.Over:
		d := demand.None()
		for _, def := range op.Defs {
			d = demand.Union(d, demandForExpr(def.Expr))
		}
		for _, e := range op.Exprs {
			d = demand.Union(d, demandForExpr(e))
		}
		return d
	case *dag.Pass:
		return downstream
	case *dag.Put:
		return demandForAssignments(op.Args, downstream)
	case *dag.Rename:
		return demandForAssignments(op.Args, downstream)
	case *dag.Scatter:
		d := demand.None()
		for _, p := range op.Paths {
			d = demand.Union(d, DemandForSeq(p, downstream))
		}
		return d
	case *dag.Scope:
		return DemandForSeq(op.Body, downstream)
	case *dag.Shape, *dag.Sort:
		return downstream
	case *dag.Summarize:
		d := demand.None()
		for _, assignment := range op.Keys {
			d = demand.Union(d, demandForExpr(assignment.RHS))
		}
		for _, assignment := range op.Aggs {
			d = demand.Union(d, demandForExpr(assignment.RHS))
		}
		return d
	case *dag.Switch:
		d := demandForExpr(op.Expr)
		for _, c := range op.Cases {
			d = demand.Union(d, demandForExpr(c.Expr))
			d = demand.Union(d, DemandForSeq(c.Path, downstream))
		}
		return d
	case *dag.Tail, *dag.Top, *dag.Uniq:
		return downstream
	case *dag.Vectorize:
		return DemandForSeq(op.Body, downstream)
	case *dag.Yield:
		d := demand.None()
		for _, e := range op.Exprs {
			d = demand.Union(d, demandForExpr(e))
		}
		return d

	case *dag.CommitMetaScan, *dag.DefaultScan, *dag.Deleter, *dag.DeleteScan, *dag.LakeMetaScan:
		return demand.None()
	case *dag.FileScan:
		d := demand.Union(downstream, demandForExpr(op.Filter))
		op.Fields = demand.Fields(d)
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
	case *dag.OverExpr:
		d := demand.None()
		for _, def := range expr.Defs {
			d = demand.Union(d, demandForExpr(def.Expr))
		}
		for _, e := range expr.Exprs {
			d = demand.Union(d, demandForExpr(e))
		}
		return d
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
	case *dag.Var:
		return demand.None()
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
