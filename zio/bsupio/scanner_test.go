package bsupio

import (
	"bytes"
	"context"
	"io"
	"runtime"
	"strconv"
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zio"
	"github.com/stretchr/testify/require"
)

func TestScannerContext(t *testing.T) {
	// We want to maximize the number of scanner goroutines running
	// concurrently, so don't call t.Parallel.
	count := runtime.GOMAXPROCS(0) + 1
	var bufs [][]byte
	var names []string
	var values []any
	// Add some BSUP streams to bufs.  The records in each stream have a type
	// unique to that stream so that they'll only validate if read with the
	// correct context.
	for i := range count {
		names = append(names, strconv.Itoa(i))
		values = append(values, i)
		rec, err := sup.NewBSUPMarshaler().MarshalCustom(names, values)
		require.NoError(t, err)
		var buf bytes.Buffer
		w := NewWriter(zio.NopCloser(&buf))
		for range 100 {
			require.NoError(t, w.Write(rec))
		}
		require.NoError(t, w.EndStream())
		require.NoError(t, w.Close())
		bufs = append(bufs, buf.Bytes())
	}
	// Create a validating BSUP reader that repeatedly reads the streams in bufs.
	var readers []io.Reader
	for range 20 {
		for j := range count {
			readers = append(readers, bytes.NewReader(bufs[j]))
		}
	}
	r := NewReaderWithOpts(super.NewContext(), io.MultiReader(readers...), ReaderOpts{
		Validate: true,
	})
	// Create a scanner and scan, validating each record.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := r.NewScanner(ctx, nil)
	require.NoError(t, err)
	for {
		batch, err := s.Pull(false)
		require.NoError(t, err)
		if batch == nil {
			break
		}
		batch.Unref()
	}
}
