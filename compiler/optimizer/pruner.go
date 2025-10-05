package optimizer

import (
	"regexp/syntax"
	"slices"
	"unicode/utf8"

	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/sup"
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
	min := dag.NewThis(field.Path{"min"})
	max := dag.NewThis(field.Path{"max"})
	if e := buildRangePruner(pred, sortKey.Key, min, max); e != nil {
		return e
	}
	return nil
}

// buildRangePruner creates a DAG comparison expression that can evalaute whether
// a value adhering to the from/to pattern can be excluded from a scan because
// the expression pred would evaluate to false for all values of fld in the
// from/to value range.  If a pruning decision cannot be reliably determined then
// the return value is nil.
func buildRangePruner(pred dag.Expr, fld field.Path, min, max *dag.ThisExpr) *dag.BinaryExpr {
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

func rangePrunerPred(op string, literal *dag.LiteralExpr, min, max *dag.ThisExpr) *dag.BinaryExpr {
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
// uses a call to the SuperSQL function "compare()" as standard comparisons
// do not handle nullsmax or cross-type comparisons (which can arise when the
// pool key value type changes).
func compare(op string, lhs, rhs dag.Expr) *dag.BinaryExpr {
	nullsMax := &dag.LiteralExpr{Kind: "LiteralExpr", Value: "true"}
	call := dag.NewCall("compare", []dag.Expr{lhs, rhs, nullsMax})
	return dag.NewBinaryExpr(op, call, &dag.LiteralExpr{Kind: "LiteralExpr", Value: "0"})
}

func literalComparison(e *dag.BinaryExpr) (*dag.ThisExpr, *dag.LiteralExpr, string) {
	switch lhs := e.LHS.(type) {
	case *dag.ThisExpr:
		if rhs, ok := e.RHS.(*dag.LiteralExpr); ok {
			return lhs, rhs, e.Op
		}
	case *dag.LiteralExpr:
		if rhs, ok := e.RHS.(*dag.ThisExpr); ok {
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

// Create a metafilter without the projection.  The projection will be
// added later when the demand is computed.
func newMetaFilter(pred dag.Expr) *dag.ScanFilter {
	e := newMetadataPruner(pred)
	if e == nil {
		return nil
	}
	return &dag.ScanFilter{Expr: e}
}

func newMetadataPruner(pred dag.Expr) dag.Expr {
	switch e := pred.(type) {
	case *dag.BinaryExpr:
		return metaPrunerBinaryExpr(e)
	case *dag.RegexpSearchExpr:
		this, ok := e.Expr.(*dag.ThisExpr)
		if !ok {
			return nil
		}
		prefix := regexpPrefix(e.Pattern)
		maxPrefix := regexpMaxPrefix(prefix)
		if prefix == "" || maxPrefix == "" {
			return nil
		}
		min := &dag.LiteralExpr{Kind: "LiteralExpr", Value: sup.QuotedString(prefix)}
		max := &dag.LiteralExpr{Kind: "LiteralExpr", Value: sup.QuotedString(maxPrefix)}
		return dag.NewBinaryExpr("and",
			compare("<=", min, dag.NewThis(append(slices.Clone(this.Path), "max"))),
			compare(">", max, dag.NewThis(append(slices.Clone(this.Path), "min"))))
	default:
		return nil
	}
}

// regexpPrefix returns the prefix of the provided regular expression (if one
// exists) only if the regular expression matches the beginning of a string.
func regexpPrefix(s string) string {
	re, err := syntax.Parse(s, syntax.Perl)
	if err != nil {
		return ""
	}
	re = re.Simplify()
	if re.Op == syntax.OpConcat &&
		len(re.Sub) >= 2 &&
		re.Sub[0].Op == syntax.OpBeginText &&
		re.Sub[1].Op == syntax.OpLiteral {
		return string(re.Sub[1].Rune)
	}
	return ""
}

func regexpMaxPrefix(s string) string {
	b := []byte(s)
	for len(b) > 0 {
		r, size := utf8.DecodeLastRune(b)
		if r == utf8.MaxRune {
			// remove last character and do this again
			b = b[:len(b)-size]
		} else {
			return string(utf8.AppendRune(b[:len(b)-size], r+1))
		}
	}
	return ""
}

func metaPrunerBinaryExpr(e *dag.BinaryExpr) dag.Expr {
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
		this, ok := e.LHS.(*dag.ThisExpr)
		if !ok {
			return nil
		}
		var literals []*dag.LiteralExpr
		switch e := e.RHS.(type) {
		case *dag.ArrayExpr:
			literals = literalsInArrayOrSet(e.Elems)
		case *dag.SetExpr:
			literals = literalsInArrayOrSet(e.Elems)
		case *dag.RecordExpr:
			for _, elem := range e.Elems {
				f, ok := elem.(*dag.Field)
				if !ok {
					return nil
				}
				l, ok := f.Value.(*dag.LiteralExpr)
				if !ok {
					return nil
				}
				literals = append(literals, l)
			}
		default:
			return nil
		}
		var ret *dag.BinaryExpr
		for _, l := range literals {
			b := metadataPrunerPred("==", this, l)
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

func literalsInArrayOrSet(elems []dag.VectorElem) []*dag.LiteralExpr {
	var literals []*dag.LiteralExpr
	for _, elem := range elems {
		val, ok := elem.(*dag.VectorValue)
		if !ok {
			return nil
		}
		l, ok := val.Expr.(*dag.LiteralExpr)
		if !ok {
			return nil
		}
		literals = append(literals, l)
	}
	return literals
}

func metadataPrunerPred(op string, this *dag.ThisExpr, literal *dag.LiteralExpr) *dag.BinaryExpr {
	min := dag.NewThis(append(slices.Clone(this.Path), "min"))
	max := dag.NewThis(append(slices.Clone(this.Path), "max"))
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
