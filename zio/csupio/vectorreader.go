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
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zbuf"
)

type VectorReader struct {
	ctx  context.Context
	sctx *super.Context

	activeReaders *atomic.Int64
	stream        *stream
	pushdown      zbuf.Pushdown
	metaFilter    *metafilter
	readerAt      io.ReaderAt
	hasClosed     bool
	vecs          []vector.Any
}

func NewVectorReader(ctx context.Context, sctx *super.Context, r io.Reader, pushdown zbuf.Pushdown) (*VectorReader, error) {
	ra, ok := r.(io.ReaderAt)
	if !ok {
		return nil, errors.New("Super Columnar requires a seekable input")
	}
	return &VectorReader{
		ctx:           ctx,
		sctx:          sctx,
		activeReaders: &atomic.Int64{},
		stream:        &stream{ctx: ctx, r: ra},
		pushdown:      pushdown,
		metaFilter:    newMetaFilter(pushdown),
		readerAt:      ra,
	}, nil
}

type metafilter struct {
	filter     expr.Evaluator
	projection field.Projection
}

func newMetaFilter(pushdown zbuf.Pushdown) *metafilter {
	if pushdown == nil {
		return nil
	}
	filter, projection, err := pushdown.MetaFilter()
	if err != nil {
		panic(err)
	}
	if filter == nil {
		return nil
	}
	return &metafilter{filter, projection}
}

func (v *VectorReader) NewConcurrentPuller() (vector.Puller, error) {
	v.activeReaders.Add(1)
	return &VectorReader{
		ctx:           v.ctx,
		sctx:          v.sctx,
		activeReaders: v.activeReaders,
		stream:        v.stream,
		pushdown:      v.pushdown,
		metaFilter:    newMetaFilter(v.pushdown),
		readerAt:      v.readerAt,
	}, nil
}

func (v *VectorReader) Pull(done bool) (vector.Any, error) {
	if done {
		return nil, v.close()
	}
	if err := v.ctx.Err(); err != nil {
		v.close()
		return nil, err
	}
	for {
		if n := len(v.vecs); n > 0 {
			// Return these last to first so v.vecs gets resued.
			vec := v.vecs[n-1]
			v.vecs = v.vecs[:n-1]
			return vec, nil
		}
		hdr, off, err := v.stream.next()
		if hdr == nil || err != nil {
			return nil, err
		}
		o, err := csup.NewObjectFromHeader(io.NewSectionReader(v.readerAt, off, math.MaxInt64), *hdr)
		if err != nil {
			return nil, err
		}
		// XXX using the query context for the metadata filter unnecessarily
		// pollutes the type context.  We should use the csup local context for
		// this filtering but this will require a little compiler refactoring to be
		// able to build runtime expressions that use different type contexts.
		if v.metaFilter == nil || !pruneObject(v.sctx, v.metaFilter, o) {
			vo := vcache.NewObjectFromCSUP(o)
			if v.pushdown.Unordered() {
				v.vecs, err = vo.FetchUnordered(v.vecs[:0], v.sctx, v.pushdown.Projection())
				if err != nil {
					return nil, err
				}
			} else {
				vec, err := vo.Fetch(v.sctx, v.pushdown.Projection())
				if err != nil {
					return nil, err
				}
				v.vecs = append(v.vecs, vec)
			}
		}
	}
}

func pruneObject(sctx *super.Context, mf *metafilter, o *csup.Object) bool {
	vals := o.ProjectMetadata(sctx, mf.projection)
	for _, val := range vals {
		if mf.filter.Eval(nil, val).Ptr().AsBool() {
			return false
		}
	}
	return true
}

func (v *VectorReader) close() error {
	if v.hasClosed {
		return nil
	}
	v.hasClosed = true
	if v.activeReaders.Add(-1) <= 0 {
		if closer, ok := v.readerAt.(io.Closer); ok {
			return closer.Close() // coffee is for closers
		}
	}
	return nil
}
