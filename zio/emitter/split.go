package emitter

import (
	"context"
	"fmt"
	"strconv"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/anyio"
)

type Split struct {
	ctx        context.Context
	dir        *storage.URI
	prefix     string
	unbuffered bool
	ext        string
	opts       anyio.WriterOpts
	writers    map[super.Type]zio.WriteCloser
	seen       map[string]struct{}
	engine     storage.Engine
}

var _ zio.Writer = (*Split)(nil)

func NewSplit(ctx context.Context, engine storage.Engine, dir *storage.URI, prefix string, unbuffered bool, opts anyio.WriterOpts) (*Split, error) {
	e := zio.Extension(opts.Format)
	if e == "" {
		return nil, fmt.Errorf("unknown format: %s", opts.Format)
	}
	if prefix != "" {
		prefix = prefix + "-"
	}
	return &Split{
		ctx:        ctx,
		dir:        dir,
		prefix:     prefix,
		unbuffered: unbuffered,
		ext:        e,
		opts:       opts,
		writers:    make(map[super.Type]zio.WriteCloser),
		seen:       make(map[string]struct{}),
		engine:     engine,
	}, nil
}

func (s *Split) Write(r super.Value) error {
	out, err := s.lookupOutput(r)
	if err != nil {
		return err
	}
	return out.Write(r)
}

func (s *Split) lookupOutput(val super.Value) (zio.WriteCloser, error) {
	typ := val.Type()
	w, ok := s.writers[typ]
	if ok {
		return w, nil
	}
	w, err := NewFileFromURI(s.ctx, s.engine, s.path(val), s.unbuffered, s.opts)
	if err != nil {
		return nil, err
	}
	s.writers[typ] = w
	return w, nil
}

// path returns the storage URI given the prefix combined with a unique ID
// to make a unique path for each Zed type.  If the _path field is present,
// we use that for the unique ID, but if the _path string appears with
// different Zed types, then we prepend it to the unique ID.
func (s *Split) path(r super.Value) *storage.URI {
	uniq := strconv.Itoa(len(s.writers))
	if _path := r.Deref("_path").AsString(); _path != "" {
		if _, ok := s.seen[_path]; ok {
			uniq = _path + "-" + uniq
		} else {
			uniq = _path
			s.seen[_path] = struct{}{}
		}
	}
	return s.dir.JoinPath(s.prefix + uniq + s.ext)
}

func (s *Split) Close() error {
	var cerr error
	for _, w := range s.writers {
		if err := w.Close(); err != nil {
			cerr = err
		}
	}
	return cerr
}
