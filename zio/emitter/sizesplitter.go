package emitter

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/bufwriter"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/anyio"
)

type sizeSplitter struct {
	ctx        context.Context
	engine     storage.Engine
	dir        *storage.URI
	prefix     string
	unbuffered bool
	opts       anyio.WriterOpts
	size       int64

	cwc countingWriteCloser
	ext string
	seq int
	zwc zio.WriteCloser
}

// NewSizeSplitter returns a zio.WriteCloser that writes to sequentially
// numbered files created by engine in dir with optional prefix and with opts,
// creating a new file after the current one reaches size bytes.  Files may
// exceed size substantially due to buffering in the underlying writer as
// determined by opts.Format.
func NewSizeSplitter(ctx context.Context, engine storage.Engine, dir *storage.URI, prefix string, unbuffered bool,
	opts anyio.WriterOpts, size int64) (zio.WriteCloser, error) {
	ext := zio.Extension(opts.Format)
	if ext == "" {
		return nil, fmt.Errorf("unknown format: %s", opts.Format)
	}
	if prefix != "" {
		prefix = prefix + "-"
	}
	return &sizeSplitter{
		ctx:        ctx,
		engine:     engine,
		dir:        dir,
		prefix:     prefix,
		unbuffered: unbuffered,
		opts:       opts,
		size:       size,
		ext:        ext,
	}, nil
}

func (s *sizeSplitter) Close() error {
	if s.zwc == nil {
		return nil
	}
	return s.zwc.Close()
}

func (s *sizeSplitter) Write(val super.Value) error {
	if s.zwc == nil {
		if err := s.nextFile(); err != nil {
			return err
		}
	}
	if err := s.zwc.Write(val); err != nil {
		return err
	}
	if s.cwc.n >= s.size {
		if err := s.zwc.Close(); err != nil {
			return err
		}
		s.zwc = nil
	}
	return nil
}

func (s *sizeSplitter) nextFile() error {
	path := s.dir.JoinPath(s.prefix + strconv.Itoa(s.seq) + s.ext)
	s.seq++
	wc, err := s.engine.Put(s.ctx, path)
	if err != nil {
		return err
	}
	if s.unbuffered {
		wc = bufwriter.New(wc)
	}
	s.cwc = countingWriteCloser{wc, 0}
	s.zwc, err = anyio.NewWriter(&s.cwc, s.opts)
	if err != nil {
		wc.Close()
		s.engine.Delete(s.ctx, path)
		return err
	}
	return nil
}

type countingWriteCloser struct {
	io.WriteCloser
	n int64
}

func (c *countingWriteCloser) Write(b []byte) (int, error) {
	n, err := c.WriteCloser.Write(b)
	c.n += int64(n)
	return n, err
}
