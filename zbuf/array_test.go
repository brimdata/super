package zbuf

import (
	"testing"

	"github.com/brimdata/super"
	"github.com/stretchr/testify/require"
)

func TestArrayWriteCopiesValueBytes(t *testing.T) {
	var a Array
	val := super.NewBytes([]byte{0})
	a.Write(val)
	copy(val.Bytes(), super.EncodeBytes([]byte{1}))
	require.Equal(t, super.NewBytes([]byte{0}), a.Values()[0])
}
