package csupio

import (
	"context"
	"errors"
	"io"
	"math"
	"sync/atomic"

	"github.com/brimdata/super"
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/vcache"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/vector"
)

type Reader struct {
	ctx  context.Context
	sctx *super.Context

	activeReaders *atomic.Int64
	stream        *stream
	pushdown      sbuf.Pushdown
	metaFilters   []*metafilter
	readerAt      io.ReaderAt
	hasClosed     bool
	vecs          [][]vector.Any
}

var _ sio.Typer = (*Reader)(nil)

func NewReader(ctx context.Context, sctx *super.Context, r io.Reader, p sbuf.Pushdown, concurrentReaders int) (*Reader, error) {
	if concurrentReaders < 1 {
		panic(concurrentReaders)
	}
	ra, ok := r.(io.ReaderAt)
	if !ok {
		return nil, errors.New("Super Columnar requires a seekable input")
	}
	var buf [1]byte
	if _, err := ra.ReadAt(buf[:], 0); err != nil && !errors.Is(err, io.EOF) {
		return nil, errors.New("Super Columnar requires a seekable input")
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
		stream:        &stream{ctx: ctx, r: ra},
		pushdown:      p,
		metaFilters:   metaFilters,
		readerAt:      ra,
		vecs:          make([][]vector.Any, concurrentReaders),
	}, nil
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
		return nil, r.close()
	}
	if err := r.ctx.Err(); err != nil {
		r.close()
		return nil, err
	}
	for {
		if k := len(r.vecs[n]); k > 0 {
			// Return these last to first so r.vecs gets resued.
			vec := r.vecs[n][k-1]
			r.vecs[n] = r.vecs[n][:k-1]
			return vec, nil
		}
		hdr, off, err := r.stream.next()
		if err != nil {
			r.close()
			return nil, err
		}
		if hdr == nil {
			return nil, r.close()
		}
		o, err := csup.NewObjectFromHeader(io.NewSectionReader(r.readerAt, off, math.MaxInt64), *hdr)
		if err != nil {
			r.close()
			return nil, err
		}
		// XXX using the query context for the metadata filter unnecessarily
		// pollutes the type context.  We should use the csup local context for
		// this filtering but this will require a little compiler refactoring to be
		// able to build runtime expressions that use different type contexts.
		if len(r.metaFilters) > 0 && pruneObject(r.sctx, r.metaFilters[n], o) {
			continue
		}
		vo := vcache.NewObjectFromCSUP(o)
		var proj field.Projection
		if r.pushdown != nil {
			proj = r.pushdown.Projection()
		}
		if r.pushdown != nil && r.pushdown.Unordered() {
			r.vecs[n], err = vo.FetchUnordered(r.vecs[n][:0], r.sctx, proj)
			if err != nil {
				r.close()
				return nil, err
			}
		} else {
			vec, err := vo.Fetch(r.sctx, proj)
			if err != nil {
				r.close()
				return nil, err
			}
			r.vecs[n] = append(r.vecs[n], vec)
		}
	}
}

func pruneObject(sctx *super.Context, mf *metafilter, o *csup.Object) bool {
	vals := o.ProjectMetadata(sctx, mf.projection)
	for _, val := range vals {
		if !mf.filter.Eval(val).Equal(super.False) {
			return false
		}
	}
	return true
}

func (r *Reader) Type() (super.Type, error) {
	return csup.FusedType(r.sctx, r.readerAt)
}

func (r *Reader) close() error {
	if r.activeReaders.Add(-1) == 0 {
		if closer, ok := r.readerAt.(io.Closer); ok {
			return closer.Close()
		}
	}
	return nil
}
