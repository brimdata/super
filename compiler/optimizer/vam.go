package optimizer

import (
	"github.com/brimdata/super/compiler/dag"
)

// IsCountByString returns whether o represents "count() by <top-level field>"
// along with the field name.
func IsCountByString(o dag.Op) (string, bool) {
	s, ok := o.(*dag.Aggregate)
	if ok && len(s.Aggs) == 1 && len(s.Keys) == 1 && isCount(s.Aggs[0]) {
		return isSingleField(s.Keys[0])
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

func isSingleField(a dag.Assignment) (string, bool) {
	lhs := fieldOf(a.LHS)
	rhs := fieldOf(a.RHS)
	if len(lhs) != 1 || len(rhs) != 1 || !lhs.Equal(rhs) {
		return "", false
	}
	return lhs[0], true
}
