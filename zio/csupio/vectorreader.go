package csupio

import (
	"context"
	"errors"
	"io"
	"sync"
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
	metaFilters   *sync.Pool
	projection    field.Projection
	readerAt      io.ReaderAt
	hasClosed     bool
}

type metafilter struct {
	filter     expr.Evaluator
	projection field.Projection
}

func NewVectorReader(ctx context.Context, sctx *super.Context, r io.Reader, pushdown zbuf.Pushdown) (*VectorReader, error) {
	ra, ok := r.(io.ReaderAt)
	if !ok {
		return nil, errors.New("Super Columnar requires a seekable input")
	}
	var mfPool *sync.Pool
	if pushdown != nil {
		filter, _, _ := pushdown.MetaFilter()
		if filter != nil {
			mfPool = &sync.Pool{
				New: func() interface{} {
					filter, projection, err := pushdown.MetaFilter()
					if err != nil {
						panic(err)
					}
					return &metafilter{
						filter:     filter,
						projection: projection,
					}
				},
			}
		}
	}
	return &VectorReader{
		ctx:           ctx,
		sctx:          sctx,
		activeReaders: &atomic.Int64{},
		stream:        &stream{r: ra},
		metaFilters:   mfPool,
		projection:    pushdown.Projection(),
		readerAt:      ra,
	}, nil
}

func (v *VectorReader) NewConcurrentPuller() (vector.Puller, error) {
	v.activeReaders.Add(1)
	return &VectorReader{
		ctx:           v.ctx,
		sctx:          v.sctx,
		activeReaders: v.activeReaders,
		metaFilters:   v.metaFilters,
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
		if v.metaFilters == nil || !pruneObject(v.sctx, v.metaFilters, o) {
			return vcache.NewObjectFromCSUP(o).Fetch(v.sctx, v.projection)
		}
	}
}

func pruneObject(sctx *super.Context, metaFilters *sync.Pool, o *csup.Object) bool {
	mf := metaFilters.Get().(*metafilter)
	defer metaFilters.Put(mf)
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
