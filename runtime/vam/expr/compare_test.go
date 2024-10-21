package expr

import (
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
	"github.com/stretchr/testify/assert"
)

type testEval struct {
	val vector.Any
}

func (t *testEval) Eval(_ vector.Any) vector.Any {
	return t.val
}

// Test that Compare.Eval handles all ops for all vector forms.
func TestCompareOpsAndForms(t *testing.T) {
	// These are all [0, 1, 2].
	lhsFlat := vector.NewUint(super.TypeUint64, []uint64{0, 1, 2}, nil)
	lhsDict := vector.NewDict(lhsFlat, []byte{0, 1, 2}, nil, nil)
	lhsView := vector.NewView([]uint32{0, 1, 2}, lhsFlat)

	// These are all [1, 1, 1].
	rhsFlat := vector.NewUint(super.TypeUint64, []uint64{1, 1, 1}, nil)
	rhsDict := vector.NewDict(rhsFlat, []byte{0, 0, 0}, nil, nil)
	rhsView := vector.NewView([]uint32{0, 1, 2}, rhsFlat)
	Const := vector.NewConst(super.NewUint64(1), 3, nil)

	cases := []struct {
		op, expected, expectedForConstLHS string
	}{
		{"==", "010", "111"},
		{"!=", "101", "000"},
		{"<", "100", "000"},
		{"<=", "110", "111"},
		{">", "001", "000"},
		{">=", "011", "111"},
	}
	for _, c := range cases {
		f := func(expected string, lhs, rhs vector.Any) {
			t.Helper()
			cmp := NewCompare(super.NewContext(), &testEval{lhs}, &testEval{rhs}, c.op)
			assert.Equal(t, expected, cmp.Eval(nil).(*vector.Bool).String(), "op: %s", c.op)
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

		// Comparing two vector.Consts yields another vector.Const.
		cmp := NewCompare(super.NewContext(), &testEval{Const}, &testEval{Const}, c.op)
		val := cmp.Eval(nil).(*vector.Const)
		assert.Equal(t, uint32(3), val.Len(), "op: %s", c.op)
		expected := super.NewBool(c.expectedForConstLHS == "111")
		assert.Equal(t, expected, val.Value(), "op: %s", c.op)
	}

}
