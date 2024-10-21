package groupby_test

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/sam/op/groupby"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/zsonio"
	"github.com/brimdata/super/ztest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupbyZtestsSpill(t *testing.T) {
	saved := groupby.DefaultLimit
	t.Cleanup(func() { groupby.DefaultLimit = saved })
	groupby.DefaultLimit = 1
	ztest.Run(t, "ztests")
}

type countReader struct {
	mu sync.Mutex
	n  int
	r  zio.Reader
}

func (cr *countReader) records() int {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	return cr.n
}

func (cr *countReader) Read() (*super.Value, error) {
	rec, err := cr.r.Read()
	if rec != nil {
		cr.mu.Lock()
		cr.n++
		cr.mu.Unlock()
	}
	return rec, err
}

type testGroupByWriter struct {
	n      int
	writer zio.Writer
	cb     func(n int)
}

func (w *testGroupByWriter) Write(val super.Value) error {
	if err := w.writer.Write(val); err != nil {
		return err
	}
	w.n += 1
	w.cb(w.n)
	return nil
}

func TestGroupbyStreamingSpill(t *testing.T) {

	// This test verifies that with sorted input, spillable groupby streams results as input arrives.
	//
	// The sorted input key is ts. The input and config parameters are carefully chosen such that:
	// - spills are not aligned with ts changes (at least some
	//   transitions from ts=n to ts=n+1 happen mid-spill)
	// - secondary keys repeat in a ts bin
	//
	// Together these conditions test that the read barrier (using
	// GroupByAggregator.maxSpillKey) does not read a key from a
	// spill before that all records for that key have been
	// written to the spill.
	//
	savedPullerBatchValues := zbuf.PullerBatchValues
	zbuf.PullerBatchValues = 1
	savedBatchSizeGroupByLimit := groupby.DefaultLimit
	groupby.DefaultLimit = 2
	defer func() {
		zbuf.PullerBatchValues = savedPullerBatchValues
		groupby.DefaultLimit = savedBatchSizeGroupByLimit
	}()

	const totRecs = 200
	const recsPerTs = 9
	const uniqueIpsPerTs = 3

	var data []string
	for i := 0; i < totRecs; i++ {
		t := i / recsPerTs
		data = append(data, fmt.Sprintf("{ts:%s,ip:1.1.1.%d}", nano.Unix(int64(t), 0), i%uniqueIpsPerTs))
	}

	runOne := func(inputSortKey string) []string {
		proc, _, err := compiler.Parse("count() by every(1s), ip")
		assert.NoError(t, err)

		zctx := super.NewContext()
		zr := zsonio.NewReader(zctx, strings.NewReader(strings.Join(data, "\n")))
		cr := &countReader{r: zr}
		var outbuf bytes.Buffer
		zw := zsonio.NewWriter(zio.NopCloser(&outbuf), zsonio.WriterOpts{})
		checker := &testGroupByWriter{
			writer: zw,
			cb: func(n int) {
				if inputSortKey != "" {
					if n == uniqueIpsPerTs {
						require.Less(t, cr.records(), totRecs)
					}
				}
			},
		}
		sortKey := order.NewSortKey(order.Asc, field.Path{inputSortKey})
		query, err := newQueryOnOrderedReader(context.Background(), zctx, proc, cr, sortKey)
		require.NoError(t, err)
		defer query.Pull(true)
		err = zbuf.CopyPuller(checker, query)
		require.NoError(t, err)
		outData := strings.Split(outbuf.String(), "\n")
		sort.Strings(outData)
		return outData
	}

	res := runOne("") // run once in non-streaming mode to have reference results to compare with.
	resStreaming := runOne("ts")
	require.Equal(t, res, resStreaming)
}

func newQueryOnOrderedReader(ctx context.Context, zctx *super.Context, program ast.Seq, reader zio.Reader, sortKey order.SortKey) (runtime.Query, error) {
	rctx := runtime.NewContext(ctx, zctx)
	q, err := compiler.CompileWithSortKey(rctx, program, reader, sortKey)
	if err != nil {
		rctx.Cancel()
		return nil, err
	}
	return q, nil
}
