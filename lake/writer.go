package lake

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/brimdata/zed/expr"
	"github.com/brimdata/zed/field"
	"github.com/brimdata/zed/lake/segment"
	"github.com/brimdata/zed/order"
	"github.com/brimdata/zed/zbuf"
	"github.com/brimdata/zed/zio"
	"github.com/brimdata/zed/zio/zngio"
	"github.com/brimdata/zed/zng"
	"golang.org/x/sync/errgroup"
)

var (
	SeekIndexStride = 64 * 1024

	// For unit testing.
	importLZ4BlockSize = zngio.DefaultLZ4BlockSize
)

const defaultCommitTimeout = time.Second * 5

// Writer is a zbuf.Writer that consumes records into memory according to
// the pools segment threshold, sorts each resulting buffer, and writes
// it as an immutable object to the storage system.  The presumption is that
// each buffer's worth of data fits into memory.
type Writer struct {
	pool        *Pool
	segments    []segment.Reference
	inputSorted bool
	ctx         context.Context
	//defs          index.Definitions
	errgroup *errgroup.Group
	records  []*zng.Record
	// XXX this is a simple double buffering model so the cloud-object
	// writer can run in parallel with the reader filling the records
	// buffer.  This can be later extended to pass a big bytes buffer
	// back and forth where the bytes buffer holds all of the record
	// data efficiently in one big backing store.
	buffer chan []*zng.Record

	memBuffered int64
	stats       ImportStats
}

//XXX NOTE: we removed the flusher logic as the callee should just put
// a timeout on the context.  We will catch that timeout here and push
// all records that have been consumed and return the commits of everything
// that made it up to the timeout.  This provides a mechanism for streaming
// microbatches with a timeout defined from above and a nice way to sync the
// timeout with the commit rather than trying to do all of this bottoms up.

// NewWriter creates a zbuf.Writer compliant writer for writing data to an
// a data pool presuming the input is not guaranteed to be sorted.
//XXX we should make another writer that takes sorted input and is a bit
// more efficient.  This other writer could have different commit triggers
// to do useful things like paritioning given the context is a rollup.
func NewWriter(ctx context.Context, pool *Pool) (*Writer, error) {
	g, ctx := errgroup.WithContext(ctx)
	ch := make(chan []*zng.Record, 1)
	ch <- nil
	return &Writer{
		pool:     pool,
		ctx:      ctx,
		errgroup: g,
		buffer:   ch,
	}, nil
}

func (w *Writer) Segments() []segment.Reference {
	return w.segments
}

func (w *Writer) newSegment() *segment.Reference {
	w.segments = append(w.segments, segment.New())
	return &w.segments[len(w.segments)-1]
}

func (w *Writer) Write(rec *zng.Record) error {
	if w.ctx.Err() != nil {
		if err := w.errgroup.Wait(); err != nil {
			return err
		}
		return w.ctx.Err()
	}
	// XXX This call leads to a ton of one-off allocations that burden the GC
	// and slow down import. We should instead copy the raw record bytes into a
	// recycled buffer and keep around an array of ts + byte-slice structs for
	// sorting.
	rec.CopyBytes()
	w.records = append(w.records, rec)
	w.memBuffered += int64(len(rec.Bytes))
	//XXX change name LogSizeThreshold
	// XXX the previous logic estimated the segment size with divide by 2...?!
	if w.memBuffered >= w.pool.Threshold {
		w.flipBuffers()
	}
	return nil
}

func (w *Writer) flipBuffers() {
	oldrecs := <-w.buffer
	recs := w.records
	w.records = oldrecs[:0]
	w.memBuffered = 0
	w.errgroup.Go(func() error {
		err := w.writeObject(w.newSegment(), recs)
		w.buffer <- recs
		return err
	})
}

func (w *Writer) Close() error {
	// Send the last write (Note: we could reorder things so we do the
	// record sort in this thread while waiting for the write to complete.)
	if len(w.records) > 0 {
		w.flipBuffers()
	}
	// Wait for any pending write to finish.
	return w.errgroup.Wait()
}

func (w *Writer) writeObject(seg *segment.Reference, recs []*zng.Record) error {
	if !w.inputSorted {
		expr.SortStable(recs, importCompareFn(w.pool))
	}
	// Set first and last key values after the sort.
	key := poolKey(w.pool.Layout)
	var err error
	seg.First, err = recs[0].Deref(key)
	if err != nil {
		seg.First = zng.Value{zng.TypeNull, nil}
	}
	seg.Last, err = recs[len(recs)-1].Deref(key)
	if err != nil {
		seg.Last = zng.Value{zng.TypeNull, nil}
	}
	writer, err := seg.NewWriter(w.ctx, w.pool.engine, w.pool.DataPath, w.pool.Layout.Order, key, SeekIndexStride)
	if err != nil {
		return err
	}
	r := zbuf.Array(recs).NewReader()
	if err := zio.CopyWithContext(w.ctx, writer, r); err != nil {
		writer.Abort()
		return err
	}
	if err := writer.Close(w.ctx); err != nil {
		return err
	}
	w.stats.Accumulate(ImportStats{
		SegmentsWritten:    1,
		RecordBytesWritten: writer.BytesWritten(),
		RecordsWritten:     int64(writer.RecordsWritten()),
	})
	return nil
}

func (w *Writer) Stats() ImportStats {
	return w.stats.Copy()
}

type ImportStats struct {
	SegmentsWritten    int64
	RecordBytesWritten int64
	RecordsWritten     int64
}

func (s *ImportStats) Accumulate(b ImportStats) {
	atomic.AddInt64(&s.SegmentsWritten, b.SegmentsWritten)
	atomic.AddInt64(&s.RecordBytesWritten, b.RecordBytesWritten)
	atomic.AddInt64(&s.RecordsWritten, b.RecordsWritten)
}

func (s *ImportStats) Copy() ImportStats {
	return ImportStats{
		SegmentsWritten:    atomic.LoadInt64(&s.SegmentsWritten),
		RecordBytesWritten: atomic.LoadInt64(&s.RecordBytesWritten),
		RecordsWritten:     atomic.LoadInt64(&s.RecordsWritten),
	}
}

func importCompareFn(pool *Pool) expr.CompareFn {
	return zbuf.NewCompareFn(poolKey(pool.Layout), pool.Layout.Order == order.Desc)
}

func poolKey(layout order.Layout) field.Path {
	if len(layout.Keys) != 0 {
		// XXX We don't yet handle multiple pool keys.
		return layout.Keys[0]
	}
	return field.New("ts")
}
