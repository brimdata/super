package optimizer

import (
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/pkg/field"
)

func maybeNewRangePruner(pred dag.Expr, sortKeys order.SortKeys) dag.Expr {
	if !sortKeys.IsNil() && pred != nil {
		return newRangePruner(pred, sortKeys.Primary())
	}
	return nil
}

// newRangePruner returns a new predicate based on the input predicate pred
// that when applied to an input value (i.e., "this") with fields from/to, returns
// true if comparisons in pred against literal values can for certain rule out
// that pred would be true for any value in the from/to range.  From/to are presumed
// to be ordered according to the order o.  This is used to prune metadata objects
// from a scan when we know the pool key range of the object could not satisfy
// the filter predicate of any of the values in the object.
func newRangePruner(pred dag.Expr, sortKey order.SortKey) dag.Expr {
	min := &dag.This{Kind: "This", Path: field.Path{"min"}}
	max := &dag.This{Kind: "This", Path: field.Path{"max"}}
	if e := buildRangePruner(pred, sortKey.Key, min, max); e != nil {
		return e
	}
	return nil
}

// buildRangePruner creates a DAG comparison expression that can evalaute whether
// a Zed value adhering to the from/to pattern can be excluded from a scan because
// the expression pred would evaluate to false for all values of fld in the
// from/to value range.  If a pruning decision cannot be reliably determined then
// the return value is nil.
func buildRangePruner(pred dag.Expr, fld field.Path, min, max *dag.This) *dag.BinaryExpr {
	e, ok := pred.(*dag.BinaryExpr)
	if !ok {
		// If this isn't a binary predicate composed of comparison operators, we
		// simply punt here.  This doesn't mean we can't optimize, because if the
		// unknown part (from here) appears in the context of an "and", then we
		// can still prune the known side of the "and" as implemented in the
		// logic below.
		return nil
	}
	switch e.Op {
	case "and":
		// For an "and", if we know either side is prunable, then we can prune
		// because both conditions are required.  So we "or" together the result
		// when both sub-expressions are valid.
		lhs := buildRangePruner(e.LHS, fld, min, max)
		rhs := buildRangePruner(e.RHS, fld, min, max)
		if lhs == nil {
			return rhs
		}
		if rhs == nil {
			return lhs
		}
		return dag.NewBinaryExpr("or", lhs, rhs)
	case "or":
		// For an "or", if we know both sides are prunable, then we can prune
		// because either condition is required.  So we "and" together the result
		// when both sub-expressions are valid.
		lhs := buildRangePruner(e.LHS, fld, min, max)
		rhs := buildRangePruner(e.RHS, fld, min, max)
		if lhs == nil || rhs == nil {
			return nil
		}
		return dag.NewBinaryExpr("and", lhs, rhs)
	case "==", "<", "<=", ">", ">=":
		this, literal, op := literalComparison(e)
		if this == nil || !fld.Equal(this.Path) {
			return nil
		}
		// At this point, we know we can definitely run a pruning decision based
		// on the literal value we found, the comparison op, and the lower/upper bounds.
		return rangePrunerPred(op, literal, min, max)
	default:
		return nil
	}
}

func rangePrunerPred(op string, literal *dag.Literal, min, max *dag.This) *dag.BinaryExpr {
	switch op {
	case "<":
		// key < CONST
		return compare("<=", literal, min)
	case "<=":
		// key <= CONST
		return compare("<", literal, min)
	case ">":
		// key > CONST
		return compare(">=", literal, max)
	case ">=":
		// key >= CONST
		return compare(">", literal, max)
	case "==":
		// key == CONST
		return dag.NewBinaryExpr("or",
			compare(">", min, literal),
			compare("<", max, literal))
	}
	panic("rangePrunerPred unknown op " + op)
}

// compare returns a DAG expression for a standard comparison operator but
// uses a call to the Zed language function "compare()" as standard comparisons
// do not handle nullsmax or cross-type comparisons (which can arise when the
// pool key value type changes).
func compare(op string, lhs, rhs dag.Expr) *dag.BinaryExpr {
	nullsMax := &dag.Literal{Kind: "Literal", Value: "true"}
	call := &dag.Call{Kind: "Call", Name: "compare", Args: []dag.Expr{lhs, rhs, nullsMax}}
	return dag.NewBinaryExpr(op, call, &dag.Literal{Kind: "Literal", Value: "0"})
}

func literalComparison(e *dag.BinaryExpr) (*dag.This, *dag.Literal, string) {
	switch lhs := e.LHS.(type) {
	case *dag.This:
		if rhs, ok := e.RHS.(*dag.Literal); ok {
			return lhs, rhs, e.Op
		}
	case *dag.Literal:
		if rhs, ok := e.RHS.(*dag.This); ok {
			return rhs, lhs, reverseComparator(e.Op)
		}
	}
	return nil, nil, ""
}

func reverseComparator(op string) string {
	switch op {
	case "==", "!=":
		return op
	case "<":
		return ">="
	case "<=":
		return ">"
	case ">":
		return "<="
	case ">=":
		return "<"
	}
	panic("unknown op")
}

func newMetadataPruner(pred dag.Expr) dag.Expr {
	e, ok := pred.(*dag.BinaryExpr)
	if !ok {
		return nil
	}
	switch e.Op {
	case "and":
		lhs := newMetadataPruner(e.LHS)
		rhs := newMetadataPruner(e.RHS)
		if lhs == nil {
			return rhs
		}
		if rhs == nil {
			return lhs
		}
		return dag.NewBinaryExpr("and", lhs, rhs)
	case "or":
		lhs := newMetadataPruner(e.LHS)
		rhs := newMetadataPruner(e.RHS)
		if lhs == nil || rhs == nil {
			return nil
		}
		return dag.NewBinaryExpr("or", lhs, rhs)
	case "==", "<", "<=", ">", ">=":
		this, literal, op := literalComparison(e)
		if this == nil {
			return nil
		}
		return metadataPrunerPred(op, this, literal)
	case "in":
		this, ok := e.LHS.(*dag.This)
		if !ok {
			return nil
		}
		var elems []dag.VectorElem
		switch e := e.RHS.(type) {
		case *dag.ArrayExpr:
			elems = e.Elems
		case *dag.SetExpr:
			elems = e.Elems
		default:
			return nil
		}
		var ret *dag.BinaryExpr
		for _, elem := range elems {
			valexpr, ok := elem.(*dag.VectorValue)
			if !ok {
				return nil
			}
			literal, ok := valexpr.Expr.(*dag.Literal)
			if !ok {
				return nil
			}
			b := metadataPrunerPred("==", this, literal)
			if ret == nil {
				ret = b
			} else {
				ret = dag.NewBinaryExpr("or", ret, b)
			}
		}
		return ret
	default:
		return nil
	}
}

func metadataPrunerPred(op string, this *dag.This, literal *dag.Literal) *dag.BinaryExpr {
	min := &dag.This{Kind: "This", Path: append(this.Path, "min")}
	max := &dag.This{Kind: "This", Path: append(this.Path, "max")}
	switch op {
	case "<":
		return compare("<", min, literal)
	case "<=":
		return compare("<=", min, literal)
	case ">":
		return compare(">", max, literal)
	case ">=":
		return compare(">=", max, literal)
	case "==":
		return dag.NewBinaryExpr("and",
			compare(">=", literal, min),
			compare("<=", literal, max))
	}
	panic("metadataPrunerPred unknown op " + op)
}
