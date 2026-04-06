package csup_test

import (
	"bytes"
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/vector"
	"github.com/stretchr/testify/require"
)

func TestDivergentUnions(t *testing.T) {
	types := super.UniqueTypes([]super.Type{super.TypeInt64, super.TypeFloat64})
	sctx := super.NewContext()
	utype, _ := sctx.LookupTypeUnion(types)
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
	w := csup.NewSerializer(sio.NopCloser(&buf))
	w.Push(u1)
	w.Push(u2)
	require.NoError(t, w.Close())
}
