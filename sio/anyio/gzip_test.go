package anyio

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGzipReaderOnlyReadsTwoBytesIfNoGzipID tests that GzipReader doesn't try
// to read more than two bytes from a non-io.ReadSeeker reader if those bytes
// aren't the gzip ID bytes.
func TestGzipReaderOnlyReadsTwoBytesIfNoGzipID(t *testing.T) {
	pr, pw := io.Pipe()
	ch := make(chan struct{})
	var writeErr error
	go func() {
		// GzipReader should return upon seeing this two-byte input.  It
		// will block (and this test will time out) if it tries to read
		// more than two bytes.
		_, writeErr = pw.Write([]byte("1\n"))
		close(ch)
	}()
	r, err := GzipReader(pr)
	require.NoError(t, err)
	require.NotNil(t, r)
	<-ch
	require.NoError(t, writeErr)
}
