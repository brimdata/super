package jsonio

import (
	"bytes"
	"math"
	"testing"

	"github.com/brimdata/super"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type nopWriteCloser struct {
	*bytes.Buffer
}

func (n nopWriteCloser) Close() error {
	return nil
}

func TestWriteEnumOutOfRangeDoesNotPanic(t *testing.T) {
	sctx := super.NewContext()
	enumType := sctx.LookupTypeEnum([]string{"ok"})
	val := super.NewValue(enumType, super.EncodeUint(math.MaxUint64))

	var out bytes.Buffer
	w := NewWriter(nopWriteCloser{Buffer: &out}, WriterOpts{})

	require.NotPanics(t, func() {
		require.NoError(t, w.Write(val))
	})
	assert.Equal(t, "\"<bad enum>\"\n", out.String())
}
