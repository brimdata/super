package anyio

import (
	"context"
	"errors"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/vector"
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
		zf, err = NewFile(ctx, sctx, sr, path, opts)
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

func NewFile(ctx context.Context, sctx *super.Context, rc io.ReadCloser, path string, opts ReaderOpts) (*sbuf.File, error) {
	r, err := GzipReader(rc)
	if err != nil {
		return nil, err
	}
	zr, err := NewReader(ctx, sctx, r, opts)
	if err != nil {
		return nil, err
	}
	return sbuf.NewFile(zr, rc, path), nil
}

// FileType returns a type for the values in the file at path.  If the file
// contains values with differing types, FileType returns a fused type.
func FileType(ctx context.Context, sctx *super.Context, engine storage.Engine, path string, opts ReaderOpts, static bool) (super.Type, error) {
	u, err := storage.ParseURI(path)
	if err != nil {
		return nil, err
	}
	r, err := engine.Get(ctx, u)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	rs, ok := isReadSeeker(r)
	if !ok {
		if static {
			return nil, errors.New("cannot get file type of non-seekable input")
		}
		return nil, nil
	}
	f, err := NewFile(ctx, sctx, r, path, opts)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// On BSD/macOS, open("/dev/stdin") dups fd 0 rather than re-opening the
	// file, so it shares stdin's file offset. Reset to 0 so a re-open reads
	// from the start instead of resuming where the last read left off.
	defer rs.Seek(0, io.SeekStart)
	if typed, ok := f.Puller.(sio.Typer); ok {
		return typed.Type()
	}
	if !static {
		return nil, nil
	}
	fuser := super.NewFuser(sctx, false)
	for {
		vec, err := f.Pull(false)
		if vec == nil || err != nil {
			return fuser.Type(), err
		}
		vector.Apply(vector.ApplyNone, func(vecs ...vector.Any) vector.Any {
			fuser.Fuse(vecs[0].Type())
			return vecs[0]
		}, vec)
	}
}

func isReadSeeker(r io.Reader) (io.ReadSeekCloser, bool) {
	rs, ok := r.(io.ReadSeekCloser)
	if !ok {
		return nil, false
	}
	_, err := rs.Seek(0, io.SeekCurrent)
	return rs, err == nil
}
