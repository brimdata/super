package storage

import (
	"io"
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
	require.Error(t, err, io.EOF)
}
