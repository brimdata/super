package optimizer

import (
	"context"

	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/pkg/field"
)

func (o *Optimizer) Vectorize(seq dag.Seq) (dag.Seq, error) {
	return walkEntries(seq, func(seq dag.Seq) (dag.Seq, error) {
		if len(seq) < 2 {
			return seq, nil
		}
		if ok, err := o.isScanWithVectors(seq[0]); !ok || err != nil {
			return seq, err
		}
		if _, ok := IsCountByString(seq[1]); ok {
			return vectorize(seq, 2), nil
		}
		if _, ok := IsSum(seq[1]); ok {
			return vectorize(seq, 2), nil
		}
		return seq, nil
	})
}

func (o *Optimizer) isScanWithVectors(op dag.Op) (bool, error) {
	scan, ok := op.(*dag.SeqScan)
	if !ok {
		return false, nil
	}
	pool, err := o.lookupPool(scan.Pool)
	if err != nil {
		return false, err
	}
	snap, err := pool.Snapshot(context.TODO(), scan.Commit)
	if err != nil {
		return false, err
	}
	objects := snap.SelectAll()
	if len(objects) == 0 {
		return false, nil
	}
	for _, obj := range objects {
		if !snap.HasVector(obj.ID) {
			return false, nil
		}
	}
	return true, nil
}

func vectorize(seq dag.Seq, n int) dag.Seq {
	return append(dag.Seq{
		&dag.Vectorize{
			Kind: "Vectorize",
			Body: seq[:n],
		},
	}, seq[n:]...)
}

// IsCountByString returns whether o represents "count() by <top-level field>"
// along with the field name.
func IsCountByString(o dag.Op) (string, bool) {
	s, ok := o.(*dag.Aggregate)
	if ok && len(s.Aggs) == 1 && len(s.Keys) == 1 && isCount(s.Aggs[0]) {
		return isSingleField(s.Keys[0])
	}
	return "", false
}

// IsSum return whether o represents "sum(<top-level field>)" along with the
// field name.
func IsSum(o dag.Op) (string, bool) {
	s, ok := o.(*dag.Aggregate)
	if ok && len(s.Aggs) == 1 && len(s.Keys) == 0 {
		if path, ok := isSum(s.Aggs[0]); ok && len(path) == 1 {
			return path[0], true
		}
	}
	return "", false
}

func isCount(a dag.Assignment) bool {
	this, ok := a.LHS.(*dag.This)
	if !ok || len(this.Path) != 1 || this.Path[0] != "count" {
		return false
	}
	agg, ok := a.RHS.(*dag.Agg)
	return ok && agg.Name == "count" && agg.Expr == nil && agg.Where == nil
}

func isSum(a dag.Assignment) (field.Path, bool) {
	this, ok := a.LHS.(*dag.This)
	if !ok || len(this.Path) != 1 || this.Path[0] != "sum" {
		return nil, false
	}
	agg, ok := a.RHS.(*dag.Agg)
	if ok && agg.Name == "sum" && agg.Where == nil {
		return isThis(agg.Expr)
	}
	return nil, false
}

func isSingleField(a dag.Assignment) (string, bool) {
	lhs := fieldOf(a.LHS)
	rhs := fieldOf(a.RHS)
	if len(lhs) != 1 || len(rhs) != 1 || !lhs.Equal(rhs) {
		return "", false
	}
	return lhs[0], true
}

func isThis(e dag.Expr) (field.Path, bool) {
	if this, ok := e.(*dag.This); ok && len(this.Path) >= 1 {
		return this.Path, true
	}
	return nil, false
}
