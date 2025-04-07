package csupio

import (
	"context"
	"errors"
	"io"
	"sync/atomic"

	"github.com/brimdata/super"
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/vcache"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zbuf"
)

type VectorReader struct {
	ctx  context.Context
	zctx *super.Context

	activeReaders *atomic.Int64
	stream        *stream
	pushdown      zbuf.Pushdown
	projection    field.Projection
	readerAt      io.ReaderAt
	hasClosed     bool
}

func NewVectorReader(ctx context.Context, zctx *super.Context, r io.Reader, pushdown zbuf.Pushdown) (*VectorReader, error) {
	ra, ok := r.(io.ReaderAt)
	if !ok {
		return nil, errors.New("Super Columnar requires a seekable input")
	}
	return &VectorReader{
		ctx:           ctx,
		zctx:          zctx,
		activeReaders: &atomic.Int64{},
		stream:        &stream{r: ra},
		projection:    pushdown.Projection(),
		readerAt:      ra,
	}, nil
}

func (v *VectorReader) NewConcurrentPuller() (vector.Puller, error) {
	v.activeReaders.Add(1)
	return &VectorReader{
		ctx:           v.ctx,
		zctx:          v.zctx, //XXX use csup object ctx?
		activeReaders: v.activeReaders,
		pushdown:      v.pushdown,
		stream:        v.stream,
		projection:    v.projection,
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
		o, err := v.stream.next()
		if o == nil || err != nil {
			return nil, err
		}
		// XXX using the query context for the metadata filter unnecessarily
		// pollutes the type context.  We should use the csup local context for
		// this filtering but this will require a little compiler refactoring to be
		// able to build runtime expressions that use different type contexts.
		if v.pushdown == nil || !pruneObject(v.zctx, v.pushdown, o.Metadata()) {
			return vcache.NewObjectFromCSUP(o).Fetch(v.zctx, v.projection)
		}
	}
}

func pruneObject(zctx *super.Context, pushdown zbuf.Pushdown, m csup.Metadata) bool {
	metaFilter, metaProjection, err := pushdown.MetaFilter()
	if err != nil {
		panic(err)
	}
	vals := csup.ProjectMetadata(zctx, m, metaProjection)
	for _, val := range vals {
		if metaFilter.Eval(nil, val).Ptr().AsBool() {
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
