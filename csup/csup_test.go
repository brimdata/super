package csup_test

import (
	"bytes"
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/fuzz"
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
