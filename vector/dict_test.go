package vector_test

import (
	"testing"

	"github.com/brimdata/zed/vector"
	"github.com/stretchr/testify/require"
)

func TestDictRebuildDropTags(t *testing.T) {
	s := vector.NewStringEmpty(0, nil)
	s.Append("foo")
	s.Append("bar")
	s.Append("baz")
	index := []byte{0, 1, 2, 0, 1, 2, 0, 1, 2}
	counts := []uint32{3, 3, 3}
	d := vector.NewDict(s, index, counts, nil)
	newIndex, counts, _, dropped := d.RebuildDropTags(0, 2)
	require.Equal(t, []uint32{0, 2, 3, 5, 6, 8}, dropped)
	require.Equal(t, []byte{0, 0, 0}, newIndex)
	require.Equal(t, []uint32{3}, counts)
}
