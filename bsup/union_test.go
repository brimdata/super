package bsup_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/superdb/super"
	"github.com/superdb/super/bsup"
	"github.com/superdb/super/sio"
	"github.com/superdb/super/vector"
)

func TestDivergentUnions(t *testing.T) {
	types := super.UniqueTypes([]super.Type{super.TypeInt64, super.TypeFloat64})
	sctx := super.NewContext()
	utype := sctx.MustLookupTypeUnion(types)
	i1 := vector.NewInt(super.TypeInt64, []int64{1, 2})
	f1 := vector.NewFloat(super.TypeFloat64, []float64{3.0})
	vecs1 := []vector.Any{i1, f1}
	tags1 := []uint32{0, 1, 0}
	u1 := vector.NewUnion(utype, tags1, vecs1)

	i2 := vector.NewInt(super.TypeInt64, []int64{1, 2})
	f2 := vector.NewFloat(super.TypeFloat64, []float64{3.0})
	vecs2 := []vector.Any{f2, i2}
	tags2 := []uint32{1, 0, 1}
	u2 := vector.NewUnion(utype, tags2, vecs2)

	var buf bytes.Buffer
	w := bsup.NewColumnWriter(sio.NopCloser(&buf))
	w.Push(u1)
	w.Push(u2)
	require.NoError(t, w.Close())
}
