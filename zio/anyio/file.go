package anyio

import (
	"context"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/optimizer/demand"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/zbuf"
)

// Open uses engine to open path for reading.  path is a local file path or a
// URI whose scheme is understood by engine.
func Open(ctx context.Context, zctx *super.Context, engine storage.Engine, path string, demandOut demand.Demand, opts ReaderOpts) (*zbuf.File, error) {
	uri, err := storage.ParseURI(path)
	if err != nil {
		return nil, err
	}
	ch := make(chan struct{})
	var zf *zbuf.File
	go func() {
		defer close(ch)
		var sr storage.Reader
		// Opening a fifo might block.
		sr, err = engine.Get(ctx, uri)
		if err != nil {
			return
		}
		// NewFile reads from sr, which might block.
		zf, err = NewFile(zctx, sr, path, demandOut, opts)
		if err != nil {
			sr.Close()
		}
	}()
	select {
	case <-ch:
		return zf, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func NewFile(zctx *super.Context, rc io.ReadCloser, path string, demandOut demand.Demand, opts ReaderOpts) (*zbuf.File, error) {
	r, err := GzipReader(rc)
	if err != nil {
		return nil, err
	}
	zr, err := NewReaderWithOpts(zctx, r, demandOut, opts)
	if err != nil {
		return nil, err
	}
	return zbuf.NewFile(zr, rc, path), nil
}
