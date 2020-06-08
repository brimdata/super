package archivestore

import (
	"context"
	"os"

	"github.com/brimsec/zq/archive"
	"github.com/brimsec/zq/pkg/nano"
	"github.com/brimsec/zq/zbuf"
	"github.com/brimsec/zq/zio/detector"
	"github.com/brimsec/zq/zng/resolver"
	"github.com/brimsec/zq/zqd/storage"
)

func Load(path string, cfg *storage.ArchiveConfig) (*Storage, error) {
	co := &archive.CreateOptions{}
	if cfg != nil && cfg.CreateOptions != nil {
		co.LogSizeThreshold = cfg.CreateOptions.LogSizeThreshold
	}
	oo := &archive.OpenOptions{}
	if cfg != nil && cfg.OpenOptions != nil {
		oo.LogFilter = cfg.OpenOptions.LogFilter
	}
	ark, err := archive.CreateOrOpenArchive(path, co, oo)
	if err != nil {
		return nil, err
	}
	return &Storage{ark: ark}, nil
}

type Storage struct {
	ark *archive.Archive
}

func (s *Storage) NativeDirection() zbuf.Direction {
	return s.ark.Meta.DataSortDirection
}

func (s *Storage) Open(ctx context.Context, span nano.Span) (zbuf.ReadCloser, error) {
	var err error
	var paths []string
	err = archive.SpanWalk(s.ark, func(si archive.SpanInfo, zardir string) error {
		if span.Overlaps(si.Span) {
			paths = append(paths, archive.ZarDirToLog(zardir))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	zctx := resolver.NewContext()
	cfg := detector.OpenConfig{Format: "zng"}
	return detector.MultiFileReader(zctx, paths, cfg), nil
}

func (s *Storage) Summary(_ context.Context) (storage.Summary, error) {
	var sum storage.Summary
	sum.Kind = storage.ArchiveStore
	return sum, archive.SpanWalk(s.ark, func(si archive.SpanInfo, zardir string) error {
		zngpath := archive.ZarDirToLog(zardir)
		sinfo, err := os.Stat(zngpath)
		if err != nil {
			return err
		}
		sum.DataBytes += sinfo.Size()
		if sum.Span.Dur == 0 {
			sum.Span = si.Span
		} else {
			sum.Span = sum.Span.Union(si.Span)
		}
		return nil
	})
}

func (s *Storage) IndexSearch(ctx context.Context, query archive.IndexQuery) (zbuf.ReadCloser, error) {
	return archive.FindReadCloser(ctx, s.ark, query, archive.AddPath(archive.DefaultAddPathField, false))
}
