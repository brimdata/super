// +build darwin dragonfly freebsd linux netbsd openbsd

package anyio

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenFifoCancelation(t *testing.T) {
	testOpenCancelation := func(path string) {
		t.Helper()
		errCh := make(chan error)
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			_, err := Open(ctx, zed.NewContext(), storage.NewFileSystem(), path, ReaderOpts{})
			errCh <- err
		}()
		time.Sleep(10 * time.Millisecond)
		cancel()
		select {
		case err := <-errCh:
			assert.ErrorIs(t, err, context.Canceled)
		case <-time.After(10 * time.Millisecond):
			t.Error("timed out waiting for error")
		}
	}

	// Opening a fifo file for reading blocks until the fifo is opened for
	// writing.  Test cancelation when Open is blocked for that reason.
	fifo := filepath.Join(t.TempDir(), "fifo")
	require.NoError(t, syscall.Mkfifo(fifo, 0600))
	testOpenCancelation(fifo)

	// Reading from a fifo file blocks if no data is available.  Test
	// cancelation when Open is blocked for that reason.
	f, err := os.OpenFile(fifo, os.O_WRONLY, 0)
	require.NoError(t, err)
	defer f.Close()
	testOpenCancelation(fifo)
}
