package bsupio

import (
	"bytes"
	"context"
	"errors"
	"io"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/superdb/super"
	"github.com/superdb/super/bsup"
	"github.com/superdb/super/pkg/field"
	"github.com/superdb/super/runtime/sam/expr"
	"github.com/superdb/super/runtime/vcache"
	"github.com/superdb/super/sbuf"
	"github.com/superdb/super/sio"
	"github.com/superdb/super/vector"
	"github.com/superdb/super/vector/vio"
)

type Reader struct {
	ctx  context.Context
	sctx *super.Context

	activeReaders *atomic.Int64
	ch            chan result
	container     *bsup.Container
	once          sync.Once
	pushdown      sbuf.Pushdown
	metaFilters   []*metafilter
	readerAt      io.ReaderAt
	vecs          [][]vector.Any
}

var _ sio.Typer = (*Reader)(nil)

func NewReader(ctx context.Context, sctx *super.Context, r io.Reader, p sbuf.Pushdown, concurrentReaders int) (*Reader, error) {
	if concurrentReaders < 1 {
		panic(concurrentReaders)
	}
	ra, ok := readerAt(r)
	if !ok {
		// XXX This is scaffolding to get everything working and passing tests.
		// We are committing this variation to main but soon thereafter we will
		// add streaming support for non-seekable inputs to disable type checking
		// and cache frames into memory one frame at a time.
		buf, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		ra = bytes.NewReader(buf)
	}

	var metaFilters []*metafilter
	if p != nil {
		filter, _, err := p.MetaFilter()
		if err != nil {
			return nil, err
		}
		if filter != nil {
			for range concurrentReaders {
				filter, projection, err := p.MetaFilter()
				if err != nil {
					return nil, err
				}
				metaFilters = append(metaFilters, &metafilter{filter, projection})
			}
		}
	}
	activeReaders := new(atomic.Int64)
	activeReaders.Store(int64(concurrentReaders))
	return &Reader{
		ctx:           ctx,
		sctx:          sctx,
		activeReaders: activeReaders,
		container:     bsup.NewContainer(sctx, ra),
		pushdown:      p,
		metaFilters:   metaFilters,
		readerAt:      ra,
		vecs:          make([][]vector.Any, concurrentReaders),
	}, nil
}

func readerAt(r io.Reader) (io.ReaderAt, bool) {
	ra, ok := r.(io.ReaderAt)
	if ok {
		var buf [1]byte
		if _, err := ra.ReadAt(buf[:], 0); err != nil && !errors.Is(err, io.EOF) {
			return nil, false
		}
		return ra, true
	}
	return nil, false
}

type metafilter struct {
	filter     expr.Evaluator
	projection field.Projection
}

func (r *Reader) Pull(done bool) (vector.Any, error) {
	return r.ConcurrentPull(done, 0)
}

func (r *Reader) ConcurrentPull(done bool, n int) (vector.Any, error) {
	if done {
		return nil, nil
	}
	if err := r.ctx.Err(); err != nil {
		return nil, err
	}
	for {
		if k := len(r.vecs[n]); k > 0 {
			// Return these last to first so r.vecs gets resued.
			vec := r.vecs[n][k-1]
			r.vecs[n] = r.vecs[n][:k-1]
			return vec, nil
		}
		reader, err := r.next()
		if reader == nil || err != nil {
			return nil, err
		}
		switch reader := reader.(type) {
		case *bsup.ColumnReader:
			// XXX using the query context for the metadata filter unnecessarily
			// pollutes the type context.  We should use the BSUP local context for
			// this filtering but this will require a little compiler refactoring to be
			// able to build runtime expressions that use different type contexts.
			if len(r.metaFilters) > 0 && pruneObject(r.sctx, r.metaFilters[n], reader) {
				continue
			}
			vo := vcache.NewReader(reader)
			var proj field.Projection
			if r.pushdown != nil {
				proj = r.pushdown.Projection()
			}
			if r.pushdown != nil && r.pushdown.Unordered() {
				r.vecs[n], err = vo.FetchUnordered(r.vecs[n][:0], r.sctx, proj)
				if err != nil {
					return nil, err
				}
			} else {
				vec, err := vo.Fetch(r.sctx, proj)
				if err != nil {
					return nil, err
				}
				if reader.IsControl() {
					vec = &vector.Control{Any: vec}
				}
				r.vecs[n] = append(r.vecs[n], vec)
			}
		case *bsup.RowReader:
			vec, err := reader.Pull()
			if err != nil {
				return nil, err
			}
			r.vecs[n] = append(r.vecs[n], vec)
		default:
			panic(reader)
		}

	}
}

type result struct {
	reader bsup.FrameReader
	err    error
}

func (r *Reader) next() (bsup.FrameReader, error) {
	r.once.Do(func() {
		r.ch = make(chan result, runtime.GOMAXPROCS(0))
		go func() {
			for {
				reader, err := r.container.Next()
				select {
				case r.ch <- result{reader, err}:
				case <-r.ctx.Done():
					return
				}
				if err != nil {
					close(r.ch)
					break
				}
			}
		}()
	})
	select {
	case r, ok := <-r.ch:
		if !ok || r.err != nil {
			if r.err == io.EOF {
				return nil, nil
			}
			return nil, r.err
		}
		return r.reader, nil
	case <-r.ctx.Done():
		return nil, r.ctx.Err()
	}
}

func pruneObject(sctx *super.Context, mf *metafilter, o *bsup.ColumnReader) bool {
	vals := o.ProjectMetadata(sctx, mf.projection)
	for _, val := range vals {
		if !mf.filter.Eval(val).Equal(super.False) {
			return false
		}
	}
	return true
}

func (r *Reader) Type() (super.Type, error) {
	return r.container.FusedType(r.sctx)
}

func NewValueReader(ctx context.Context, sctx *super.Context, r io.Reader) (sio.ReadCloser, error) {
	puller, err := NewReader(ctx, sctx, r, nil, 1)
	if err != nil {
		return nil, err
	}
	return &valueReader{
		Reader: sbuf.NewReader(puller),
		puller: puller,
	}, nil
}

type valueReader struct {
	sio.Reader
	puller *Reader
}

func (v *valueReader) Pull(done bool) (sbuf.Batch, error) {
	vec, err := v.puller.Pull(done)
	if vec == nil || err != nil {
		return nil, err
	}
	return sbuf.Materialize(vec), nil
}

func (v *valueReader) Close() error {
	return nil
}

func (v *valueReader) Progress() vio.Progress {
	//XXX implements sbuf.Scanner
	return vio.Progress{}
}
