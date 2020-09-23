package archive

import (
	"context"
	"errors"

	"github.com/brimsec/zq/microindex"
	"github.com/brimsec/zq/pkg/iosrc"
	"github.com/brimsec/zq/zbuf"
	"github.com/brimsec/zq/zcode"
	"github.com/brimsec/zq/zng"
	"github.com/brimsec/zq/zng/resolver"
	"github.com/brimsec/zq/zqe"
)

// statReadCloser implements zbuf.ReadCloser.
type statReadCloser struct {
	ctx    context.Context
	cancel context.CancelFunc
	ark    *Archive
	zctx   *resolver.Context
	recs   chan *zng.Record
	err    error

	chunkBuilder  *zng.Builder
	indexBuilders map[string]*zng.Builder
}

func (s *statReadCloser) Read() (*zng.Record, error) {
	select {
	case r, ok := <-s.recs:
		if !ok {
			return nil, s.err
		}
		return r, nil
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	}
}

func (s *statReadCloser) Close() error {
	s.cancel()
	return nil
}

func (s *statReadCloser) chunkRecord(si SpanInfo, zardir iosrc.URI) error {
	fi, err := iosrc.Stat(s.ctx, ZarDirToLog(zardir))
	if err != nil {
		return err
	}

	if s.chunkBuilder == nil {
		s.chunkBuilder = zng.NewBuilder(s.zctx.MustLookupTypeRecord([]zng.Column{
			zng.NewColumn("type", zng.TypeString),
			zng.NewColumn("log_id", zng.TypeString),
			zng.NewColumn("first", zng.TypeTime),
			zng.NewColumn("last", zng.TypeTime),
			zng.NewColumn("size", zng.TypeUint64),
			zng.NewColumn("record_count", zng.TypeUint64),
		}))
	}

	rec := s.chunkBuilder.Build(
		zng.EncodeString("chunk"),
		zng.EncodeString(string(si.LogID)),
		zng.EncodeTime(si.First),
		zng.EncodeTime(si.Last),
		zng.EncodeUint(uint64(fi.Size())),
		zng.EncodeUint(uint64(si.RecordCount)),
	).Keep()
	select {
	case s.recs <- rec:
		return nil
	case <-s.ctx.Done():
		return s.ctx.Err()
	}
}

func (s *statReadCloser) indexRecord(si SpanInfo, zardir iosrc.URI, indexPath string) error {
	info, err := microindex.Stat(s.ctx, zardir.AppendPath(indexPath))
	if err != nil {
		if errors.Is(err, zqe.E(zqe.NotFound)) {
			return nil
		}
		return err
	}

	if s.indexBuilders == nil {
		s.indexBuilders = make(map[string]*zng.Builder)
	}
	if s.indexBuilders[indexPath] == nil {
		keycols := make([]zng.Column, len(info.Keys))
		for i, k := range info.Keys {
			keycols[i] = zng.Column{
				Name: k.Name,
				Type: zng.TypeString,
			}
		}
		keyrec := s.zctx.MustLookupTypeRecord(keycols)

		s.indexBuilders[indexPath] = zng.NewBuilder(s.zctx.MustLookupTypeRecord([]zng.Column{
			zng.NewColumn("type", zng.TypeString),
			zng.NewColumn("log_id", zng.TypeString),
			zng.NewColumn("first", zng.TypeTime),
			zng.NewColumn("last", zng.TypeTime),
			zng.NewColumn("index_id", zng.TypeString),
			zng.NewColumn("size", zng.TypeUint64),
			zng.NewColumn("record_count", zng.TypeUint64),
			zng.NewColumn("keys", keyrec),
		}))
	}

	if len(s.indexBuilders[indexPath].Type.Columns) != 7+len(info.Keys) {
		return zqe.E("key record differs in index files %s %s", indexPath, si.LogID)
	}
	var keybytes zcode.Bytes
	for _, k := range info.Keys {
		keybytes = zcode.AppendPrimitive(keybytes, zng.EncodeString(k.TypeName))
	}

	rec := s.indexBuilders[indexPath].Build(
		zng.EncodeString("index"),
		zng.EncodeString(string(si.LogID)),
		zng.EncodeTime(si.First),
		zng.EncodeTime(si.Last),
		zng.EncodeString(indexPath),
		zng.EncodeUint(uint64(info.Size)),
		zng.EncodeUint(uint64(si.RecordCount)),
		keybytes,
	).Keep()
	select {
	case s.recs <- rec:
		return nil
	case <-s.ctx.Done():
		return s.ctx.Err()
	}
}

func (s *statReadCloser) run() {
	defer close(s.recs)

	s.err = SpanWalk(s.ctx, s.ark, func(si SpanInfo, zardir iosrc.URI) error {
		if err := s.chunkRecord(si, zardir); err != nil {
			return err
		}
		if dirents, err := s.ark.dataSrc.ReadDir(s.ctx, zardir); err == nil {
			for _, e := range dirents {
				if e.IsDir() {
					continue
				}
				if err := s.indexRecord(si, zardir, e.Name()); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func Stat(ctx context.Context, zctx *resolver.Context, ark *Archive) (zbuf.ReadCloser, error) {
	ctx, cancel := context.WithCancel(ctx)
	s := &statReadCloser{
		ctx:    ctx,
		cancel: cancel,
		ark:    ark,
		zctx:   zctx,
		recs:   make(chan *zng.Record),
	}
	go s.run()
	return s, nil
}
