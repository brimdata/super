package anyio

import (
	"context"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/sbuf"
)

// Open uses engine to open path for reading.  path is a local file path or a
// URI whose scheme is understood by engine.
func Open(ctx context.Context, sctx *super.Context, engine storage.Engine, path string, opts ReaderOpts) (*sbuf.File, error) {
	uri, err := storage.ParseURI(path)
	if err != nil {
		return nil, err
	}
	ch := make(chan struct{})
	var zf *sbuf.File
	go func() {
		defer close(ch)
		var sr storage.Reader
		// Opening a fifo might block.
		sr, err = engine.Get(ctx, uri)
		if err != nil {
			return
		}
		// NewFile reads from sr, which might block.
		zf, err = NewFile(sctx, sr, path, opts)
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

func NewFile(sctx *super.Context, rc io.ReadCloser, path string, opts ReaderOpts) (*sbuf.File, error) {
	r, err := GzipReader(rc)
	if err != nil {
		return nil, err
	}
	zr, err := NewReaderWithOpts(sctx, r, opts)
	if err != nil {
		return nil, err
	}
	return sbuf.NewFile(zr, rc, path), nil
}
