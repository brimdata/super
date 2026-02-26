package anyio

import (
	"context"
	"io"
	"math"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/runtime/sam/expr/agg"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sio/arrowio"
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

// FileType returns a type for the values in the file at path.  If the file
// contains values with differing types, FileType returns a fused type.  If
// FileType must read values to compute a fused type, it reads at most
// sampleSize values or the entire file if sampleSize is negative, and it
// returns a nil type if sampleSize is zero or the file is empty.
func FileType(ctx context.Context, sctx *super.Context, engine storage.Engine, path string, opts ReaderOpts, sampleSize int) (super.Type, error) {
	u, err := storage.ParseURI(path)
	if err != nil {
		return nil, err
	}
	r, err := engine.Get(ctx, u)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var b [1]byte
	if _, err := r.ReadAt(b[:], 0); err != nil {
		// r can't seek so it's a fifo or pipe.
		return nil, nil
	}
	f, err := NewFile(sctx, r, path, opts)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	switch r := f.Reader.(type) {
	case *arrowio.Reader:
		return r.Type(), nil
	}
	if sampleSize < 0 {
		sampleSize = math.MaxInt
	}
	fuser := agg.NewFuser(sctx)
	for range sampleSize {
		val, err := f.Read()
		if val == nil || err != nil {
			return fuser.Type(), err
		}
		fuser.Fuse(val.Type())
	}
	return fuser.Type(), err
}
