package vngio

import (
	"context"
	"errors"
	"io"
	"sync/atomic"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/vcache"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vng"
)

type VectorReader struct {
	ctx  context.Context
	zctx *super.Context

	activeReaders *atomic.Int64
	nextObject    *atomic.Int64
	objects       []*vng.Object
	projection    vcache.Path
	readerAt      io.ReaderAt
}

func NewVectorReader(ctx context.Context, zctx *super.Context, r io.Reader, fields []field.Path) (*VectorReader, error) {
	ra, ok := r.(io.ReaderAt)
	if !ok {
		return nil, errors.New("Super Columnar requires a seekable input")
	}
	objects, err := readObjects(ra)
	if err != nil {
		return nil, err
	}
	return &VectorReader{
		ctx:           ctx,
		zctx:          zctx,
		activeReaders: &atomic.Int64{},
		nextObject:    &atomic.Int64{},
		objects:       objects,
		projection:    vcache.NewProjection(fields),
		readerAt:      ra,
	}, nil
}

func (v *VectorReader) NewConcurrentPuller() vector.Puller {
	v.activeReaders.Add(1)
	return v
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
	return vcache.NewObjectFromVNG(o).Fetch(v.zctx, v.projection)
}

func (v *VectorReader) close() error {
	if v.activeReaders.Add(-1) <= 0 {
		if closer, ok := v.readerAt.(io.Closer); ok {
			return closer.Close() // coffee is for closers
		}
	}
	return nil
}
