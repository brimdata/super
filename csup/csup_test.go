package csup_test

import (
	"bytes"
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/fuzz"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/csupio"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
	"github.com/stretchr/testify/require"
)

func FuzzCSUPRoundtripGen(f *testing.F) {
	f.Fuzz(func(t *testing.T, b []byte) {
		bytesReader := bytes.NewReader(b)
		context := super.NewContext()
		types := fuzz.GenTypes(bytesReader, context, 3)
		values := fuzz.GenValues(bytesReader, context, types)
		roundtrip(t, values)
	})
}

func FuzzCSUPRoundtripBytes(f *testing.F) {
	f.Fuzz(func(t *testing.T, b []byte) {
		values, err := fuzz.ReadBSUP(b)
		if err != nil {
			t.Skipf("%v", err)
		}
		roundtrip(t, values)
	})
}

func roundtrip(t *testing.T, valuesIn []super.Value) {
	var buf bytes.Buffer
	fuzz.WriteCSUP(t, valuesIn, &buf)
	valuesOut, err := fuzz.ReadCSUP(buf.Bytes(), nil)
	require.NoError(t, err)
	fuzz.CompareValues(t, valuesIn, valuesOut)
}

func TestCSUPBatchBug(t *testing.T) {
	var b bytes.Buffer
	w := csup.NewSerializer(sio.NopCloser(&b))
	sctx := super.NewContext()
	v1, err := sup.ParseValue(sctx, `{a: [1,2,3]}`)
	require.NoError(t, err)
	val2, err := sup.ParseValue(sctx, `{a:[4,5]}`)
	require.NoError(t, err)
	err = w.Push(valToVec(sctx, v1))
	require.NoError(t, err)
	err = w.Push(valToVec(sctx, val2))
	err = w.Close()
	require.NoError(t, err)
	r, err := csupio.NewReader(sctx, bytes.NewReader(b.Bytes()), nil)
	require.NoError(t, err)
	val, err := r.Read()
	require.NoError(t, err)
	require.Equal(t, "{a:[1,2,3]}", sup.String(val))
	val, err = r.Read()
	require.NoError(t, err)
	require.Equal(t, "{a:[4,5]}", sup.String(val))
}

func valToVec(sctx *super.Context, val super.Value) vector.Any {
	b := vector.NewDynamicValueBuilder()
	b.Write(val)
	return b.Build(sctx)
}
