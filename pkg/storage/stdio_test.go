package storage

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStdinGetReturnsWorkingReaderAfterClose(t *testing.T) {
	e := NewStdioEngine()
	u := MustParseURI("stdio:stdin")
	r, err := e.Get(t.Context(), u)
	require.NoError(t, err)
	require.NoError(t, r.Close())
	r, err = e.Get(t.Context(), u)
	require.NoError(t, err)
	_, err = r.Read(nil)
	require.NoError(t, err, "zero-length read should succeed")
}
