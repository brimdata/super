package csupio

import (
	"context"
	"errors"
	"io"
	"slices"
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
	zctx *super.Context

	activeReaders *atomic.Int64
	nextObject    *atomic.Int64
	objects       []*csup.Object
	projection    vcache.Path
	readerAt      io.ReaderAt
	hasClosed     bool
}

func NewVectorReader(ctx context.Context, zctx *super.Context, r io.Reader, fields []field.Path, pruner zbuf.Filter) (*VectorReader, error) {
	ra, ok := r.(io.ReaderAt)
	if !ok {
		return nil, errors.New("Super Columnar requires a seekable input")
	}
	objects, err := readObjects(ra)
	if err != nil {
		return nil, err
	}
	var evaluator expr.Evaluator
	if pruner != nil {
		evaluator, err = pruner.AsEvaluator()
		if err != nil {
			return nil, err
		}
	}
	return &VectorReader{
		ctx:           ctx,
		zctx:          zctx,
		activeReaders: &atomic.Int64{},
		nextObject:    &atomic.Int64{},
		objects:       filterObjects(zctx, evaluator, objects),
		projection:    vcache.NewProjection(fields),
		readerAt:      ra,
	}, nil
}

func (v *VectorReader) NewConcurrentPuller() (vector.Puller, error) {
	v.activeReaders.Add(1)
	return &VectorReader{
		ctx:           v.ctx,
		zctx:          v.zctx,
		activeReaders: v.activeReaders,
		nextObject:    v.nextObject,
		objects:       v.objects,
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
	n := int(v.nextObject.Add(1) - 1)
	if n >= len(v.objects) {
		return nil, v.close()
	}
	o := v.objects[n]
	return vcache.NewObjectFromCSUP(o).Fetch(v.zctx, v.projection)
}

func filterObjects(zctx *super.Context, pruner expr.Evaluator, objects []*csup.Object) []*csup.Object {
	if pruner == nil {
		return objects
	}
	return slices.DeleteFunc(objects, func(o *csup.Object) bool {
		return pruneObject(zctx, pruner, o.Metadata())
	})
}

func pruneObject(zctx *super.Context, pruner expr.Evaluator, m csup.Metadata) bool {
	for _, val := range csup.MetadataValues(zctx, m) {
		if pruner.Eval(nil, val).Ptr().AsBool() {
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
