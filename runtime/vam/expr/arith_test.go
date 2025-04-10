package expr

import (
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
	"github.com/stretchr/testify/assert"
)

// Test that Arith.Eval handles all ops for all vector forms.
func TestArithOpsAndForms(t *testing.T) {
	// These are all [0, 1, 2].
	lhsFlat := vector.NewInt(super.TypeInt64, []int64{0, 1, 2}, nil)
	lhsDict := vector.NewDict(lhsFlat, []byte{0, 1, 2}, nil, nil)
	lhsView := vector.Pick(lhsFlat, []uint32{0, 1, 2})

	// These are all [1, 1, 1].
	rhsFlat := vector.NewInt(super.TypeInt64, []int64{1, 1, 1}, nil)
	rhsDict := vector.NewDict(rhsFlat, []byte{0, 0, 0}, nil, nil)
	rhsView := vector.Pick(rhsFlat, []uint32{0, 1, 2})
	Const := vector.NewConst(super.NewInt64(1), 3, nil)

	cases := []struct {
		op                            string
		expected, expectedForConstLHS []int64
	}{
		{"+", []int64{1, 2, 3}, []int64{2, 2, 2}},
		{"-", []int64{-1, 0, 1}, []int64{0, 0, 0}},
		{"*", []int64{0, 1, 2}, []int64{1, 1, 1}},
		{"/", []int64{0, 1, 2}, []int64{1, 1, 1}},
		{"%", []int64{0, 0, 0}, []int64{0, 0, 0}},
	}
	for _, c := range cases {
		f := func(expected []int64, lhs, rhs vector.Any) {
			t.Helper()
			cmp := NewArith(super.NewContext(), &testEval{lhs}, &testEval{rhs}, c.op)
			assert.Equal(t, expected, cmp.Eval(nil).(*vector.Int).Values, "op: %s", c.op)
		}

		f(c.expected, lhsFlat, rhsFlat)
		f(c.expected, lhsFlat, rhsDict)
		f(c.expected, lhsFlat, rhsView)
		f(c.expected, lhsFlat, Const)

		f(c.expected, lhsDict, rhsFlat)
		f(c.expected, lhsDict, rhsDict)
		f(c.expected, lhsDict, rhsView)
		f(c.expected, lhsDict, Const)

		f(c.expected, lhsView, rhsFlat)
		f(c.expected, lhsView, rhsDict)
		f(c.expected, lhsView, rhsView)
		f(c.expected, lhsView, Const)

		f(c.expectedForConstLHS, Const, rhsFlat)
		f(c.expectedForConstLHS, Const, rhsDict)
		f(c.expectedForConstLHS, Const, rhsView)

		// Arithmetic on two vector.Consts yields another vector.Const.
		cmp := NewArith(super.NewContext(), &testEval{Const}, &testEval{Const}, c.op)
		val := cmp.Eval(nil).(*vector.Const)
		assert.Equal(t, uint32(3), val.Len(), "op: %s", c.op)
		expected := super.NewInt64(c.expectedForConstLHS[0])
		assert.Equal(t, expected, val.Value(), "op: %s", c.op)
	}

}
