package aggregate_test

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler"
	"github.com/brimdata/super/compiler/parser"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/sam/op/aggregate"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/supio"
	"github.com/brimdata/super/ztest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAggregateZtestsSpill(t *testing.T) {
	saved := aggregate.DefaultLimit
	t.Cleanup(func() { aggregate.DefaultLimit = saved })
	aggregate.DefaultLimit = 1
	ztest.Run(t, "../../../ztests/op/aggregate")
}

type countReader struct {
	r zio.Reader
	n atomic.Int64
}

var _ zbuf.ScannerAble = (*countReader)(nil)

func (c *countReader) NewScanner(context.Context, zbuf.Pushdown) (zbuf.Scanner, error) {
	return c, nil
}

func (*countReader) Progress() zbuf.Progress {
	panic("unused")
}

func (c *countReader) Pull(bool) (zbuf.Batch, error) {
	val, err := c.r.Read()
	if val == nil || err != nil {
		return nil, err
	}
	// Feed values to the caller one at a time.
	c.n.Add(1)
	return zbuf.NewArray([]super.Value{val.Copy()}), nil
}

func (*countReader) Read() (*super.Value, error) {
	panic("unused")
}

type testAggregateWriter struct {
	n      int
	writer zio.Writer
	cb     func(n int)
}

func (w *testAggregateWriter) Write(val super.Value) error {
	if err := w.writer.Write(val); err != nil {
		return err
	}
	w.n += 1
	w.cb(w.n)
	return nil
}

func TestAggregateStreamingSpill(t *testing.T) {
	// This test verifies that with sorted input, spillable aggregate streams results as input arrives.
	//
	// The sorted input key is ts. The input and config parameters are carefully chosen such that:
	// - spills are not aligned with ts changes (at least some
	//   transitions from ts=n to ts=n+1 happen mid-spill)
	// - secondary keys repeat in a ts bin
	//
	// Together these conditions test that the read barrier (using
	// Aggregator.maxSpillKey) does not read a key from a
	// spill before that all records for that key have been
	// written to the spill.
	//
	savedPullerBatchValues := zbuf.PullerBatchValues
	zbuf.PullerBatchValues = 1
	savedDefaultLimit := aggregate.DefaultLimit
	aggregate.DefaultLimit = 2
	defer func() {
		zbuf.PullerBatchValues = savedPullerBatchValues
		aggregate.DefaultLimit = savedDefaultLimit
	}()

	const totRecs = 200
	const recsPerTs = 9
	const uniqueIpsPerTs = 3

	var data []string
	for i := range totRecs {
		t := i / recsPerTs
		data = append(data, fmt.Sprintf("{ts:%s,ip:1.1.1.%d}", nano.Unix(int64(t), 0), i%uniqueIpsPerTs))
	}

	runOne := func(inputSortKey string) []string {
		ast, err := parser.ParseQuery("count() by every(1s), ip")
		assert.NoError(t, err)

		sctx := super.NewContext()
		zr := supio.NewReader(sctx, strings.NewReader(strings.Join(data, "\n")))
		cr := &countReader{r: zr}
		var outbuf bytes.Buffer
		zw := supio.NewWriter(zio.NopCloser(&outbuf), supio.WriterOpts{})
		checker := &testAggregateWriter{
			writer: zw,
			cb: func(n int) {
				if inputSortKey != "" {
					if n == uniqueIpsPerTs {
						require.Less(t, cr.n.Load(), int64(totRecs))
					}
				}
			},
		}
		sortKey := order.NewSortKey(order.Asc, field.Path{inputSortKey})
		query, err := newQueryOnOrderedReader(context.Background(), sctx, ast, cr, sortKey)
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

func newQueryOnOrderedReader(ctx context.Context, sctx *super.Context, ast *parser.AST, reader zio.Reader, sortKey order.SortKey) (runtime.Query, error) {
	rctx := runtime.NewContext(ctx, sctx)
	q, err := compiler.CompileWithSortKey(rctx, ast, reader, sortKey)
	if err != nil {
		rctx.Cancel()
		return nil, err
	}
	return q, nil
}
