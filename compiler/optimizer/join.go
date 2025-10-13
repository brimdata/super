package optimizer

import (
	"reflect"

	"github.com/brimdata/super/compiler/dag"
)

func replaceJoinWithHashJoin(seq dag.Seq) {
	walkT(reflect.ValueOf(seq), func(op dag.Op) dag.Op {
		j, ok := op.(*dag.JoinOp)
		if !ok {
			return op
		}
		left, right, ok := equiJoinKeyExprs(j.Cond, j.LeftAlias, j.RightAlias)
		if !ok {
			return op
		}
		return &dag.HashJoinOp{
			Kind:       "HashJoinOp",
			Style:      j.Style,
			LeftAlias:  j.LeftAlias,
			RightAlias: j.RightAlias,
			LeftKey:    left,
			RightKey:   right,
		}
	})
}

func liftFilterConvertCrossJoin(seq dag.Seq) dag.Seq {
	var filter *dag.FilterOp
	var i int
	for i = range len(seq) - 3 {
		_, isfork := seq[i].(*dag.ForkOp)
		_, _, _, isjoin := isJoin(seq[i+1])
		var isfilter bool
		filter, isfilter = seq[i+2].(*dag.FilterOp)
		if isfork && isjoin && isfilter {
			break
		}
	}
	if filter == nil {
		return seq
	}
	in := splitPredicate(filter.Expr)
	var exprs []dag.Expr
	for _, e := range in {
		if b, ok := e.(*dag.BinaryExpr); ok && b.Op == "==" && convertCrossJoinToHashJoin(seq[i:], b.LHS, b.RHS) {
			continue
		}
		exprs = append(exprs, e)
	}
	if len(exprs) == 0 {
		seq.Delete(i+2, i+3)
	} else if len(exprs) != len(in) {
		seq[i+2] = dag.NewFilterOp(buildConjunction(exprs))
	}
	return seq
}

func convertCrossJoinToHashJoin(seq dag.Seq, lhs, rhs dag.Expr) bool {
	fork, isfork := seq[0].(*dag.ForkOp)
	leftAlias, rightAlias, style, isjoin := isJoin(seq[1])
	if !isfork || !isjoin {
		return false
	}
	if len(fork.Paths) != 2 {
		panic(fork)
	}
	lhsFirst, lok := firstThisPathComponent(lhs)
	rhsFirst, rok := firstThisPathComponent(rhs)
	if !lok || !rok {
		return false
	}
	lhs, rhs = dag.CopyExpr(lhs), dag.CopyExpr(rhs)
	stripFirstThisPathComponent(lhs)
	stripFirstThisPathComponent(rhs)
	if lhsFirst == rhsFirst {
		if lhsFirst == leftAlias {
			return convertCrossJoinToHashJoin(fork.Paths[0], lhs, rhs)
		}
		if lhsFirst == rightAlias {
			return convertCrossJoinToHashJoin(fork.Paths[1], lhs, rhs)
		}
		return false
	}
	if style != "cross" {
		return false
	}
	if lhsFirst != leftAlias {
		lhsFirst, rhsFirst = rhsFirst, lhsFirst
		lhs, rhs = rhs, lhs
	}
	if lhsFirst != leftAlias || rhsFirst != rightAlias {
		return false
	}
	seq[1] = &dag.HashJoinOp{
		Kind:       "HashJoinOp",
		Style:      "inner",
		LeftAlias:  leftAlias,
		RightAlias: rightAlias,
		LeftKey:    lhs,
		RightKey:   rhs,
	}
	return true
}

func equiJoinKeyExprs(e dag.Expr, leftAlias, rightAlias string) (left, right dag.Expr, ok bool) {
	b, ok := e.(*dag.BinaryExpr)
	if !ok || b.Op != "==" {
		return nil, nil, false
	}
	lhsFirst, ok := firstThisPathComponent(b.LHS)
	if !ok {
		return nil, nil, false
	}
	rhsFirst, ok := firstThisPathComponent(b.RHS)
	if !ok {
		return nil, nil, false
	}
	lhs, rhs := b.LHS, b.RHS
	if lhsFirst != leftAlias {
		lhsFirst, rhsFirst = rhsFirst, lhsFirst
		lhs, rhs = rhs, lhs
	}
	if lhsFirst != leftAlias || rhsFirst != rightAlias {
		return nil, nil, false
	}
	stripFirstThisPathComponent(lhs)
	stripFirstThisPathComponent(rhs)
	return lhs, rhs, true
}

// firstThisPathComponent returns the first component common to every dag.This.Path
// in e and a Boolean indicating whether such a common first component exists.
func firstThisPathComponent(e dag.Expr) (prefix string, ok bool) {
	walkT(reflect.ValueOf(e), func(t dag.ThisExpr) dag.ThisExpr {
		if prefix == "" {
			prefix = t.Path[0]
			ok = true
		} else if prefix != t.Path[0] {
			ok = false
		}
		return t
	})
	return prefix, ok
}

// stripFirstThisPathComponent removes the first component of every dag.This.Path in e.
func stripFirstThisPathComponent(e dag.Expr) {
	walkT(reflect.ValueOf(e), func(t dag.ThisExpr) dag.ThisExpr {
		t.Path = t.Path[1:]
		return t
	})
}
