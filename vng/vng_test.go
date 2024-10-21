package vng_test

import (
	"bytes"
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/optimizer/demand"
	"github.com/brimdata/super/fuzz"
	"github.com/stretchr/testify/require"
)

func FuzzVngRoundtripGen(f *testing.F) {
	f.Fuzz(func(t *testing.T, b []byte) {
		bytesReader := bytes.NewReader(b)
		context := super.NewContext()
		types := fuzz.GenTypes(bytesReader, context, 3)
		values := fuzz.GenValues(bytesReader, context, types)
		roundtrip(t, values)
	})
}

func FuzzVngRoundtripBytes(f *testing.F) {
	f.Fuzz(func(t *testing.T, b []byte) {
		values, err := fuzz.ReadZNG(b)
		if err != nil {
			t.Skipf("%v", err)
		}
		roundtrip(t, values)
	})
}

func roundtrip(t *testing.T, valuesIn []super.Value) {
	var buf bytes.Buffer
	fuzz.WriteVNG(t, valuesIn, &buf)
	valuesOut, err := fuzz.ReadVNG(buf.Bytes(), demand.All())
	require.NoError(t, err)
	fuzz.CompareValues(t, valuesIn, valuesOut)
}
