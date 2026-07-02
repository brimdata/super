package anyio

import (
	"context"
	"io"
	"math"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/runtime/sam/expr/agg"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sio"
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
	zr, err := NewReader(sctx, r, opts)
	if err != nil {
		return nil, err
	}
	return sbuf.NewFile(zr, rc, path), nil
}

// FileType returns a type for the values in the file at path.  If the file
// contains values with differing types, FileType returns a fused type.  If
// FileType must read values to compute a fused type, it reads at most
// sampleSize values or the entire file if sampleSize is less than 1, and it
// returns a nil type if the file is empty.
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
	rs, ok := r.(io.ReadSeekCloser)
	if !ok {
		return nil, nil
	}
	if _, err := rs.Seek(0, io.SeekCurrent); err != nil {
		return nil, nil
	}
	f, err := NewFile(sctx, r, path, opts)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// On BSD/macOS, open("/dev/stdin") dups fd 0 rather than re-opening the
	// file, so it shares stdin's file offset. Reset to 0 so a re-open reads
	// from the start instead of resuming where the last read left off.
	defer rs.Seek(0, io.SeekStart)
	if typed, ok := f.Reader.(sio.Typer); ok {
		return typed.Type()
	}
	if sampleSize < 1 {
		sampleSize = math.MaxInt
	}
	// XXX this should pass super true when type checker can handle it
	fuser := agg.NewFuser(sctx, false)
	for range sampleSize {
		val, err := f.Read()
		if val == nil || err != nil {
			return fuser.Type(), err
		}
		fuser.Fuse(val.Type())
	}
	return fuser.Type(), err
}
