package optimizer

import (
	"reflect"

	"github.com/brimdata/super/compiler/dag"
)

func replaceJoinWithHashJoin(seq dag.Seq) {
	walkT(reflect.ValueOf(seq), func(op dag.Op) dag.Op {
		j, ok := op.(*dag.Join)
		if !ok {
			return op
		}
		left, right, ok := equiJoinKeyExprs(j.Cond, j.LeftAlias, j.RightAlias)
		if !ok {
			return op
		}
		return &dag.HashJoin{
			Kind:       "HashJoin",
			Style:      j.Style,
			LeftAlias:  j.LeftAlias,
			RightAlias: j.RightAlias,
			LeftKey:    left,
			RightKey:   right,
		}
	})
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
	walkT(reflect.ValueOf(e), func(t dag.This) dag.This {
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
	walkT(reflect.ValueOf(e), func(t dag.This) dag.This {
		t.Path = t.Path[1:]
		return t
	})
}
