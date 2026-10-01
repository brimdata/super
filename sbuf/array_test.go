package sbuf

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/superdb/super"
)

func TestArrayWriteCopiesValueBytes(t *testing.T) {
	var a Array
	val := super.NewBytes([]byte{0})
	a.Write(val)
	copy(val.Bytes(), super.EncodeBytes([]byte{1}))
	require.Equal(t, super.NewBytes([]byte{0}), a.Values()[0])
}
