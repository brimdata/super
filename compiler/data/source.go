package data

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast/dag"
	"github.com/brimdata/super/compiler/optimizer/demand"
	"github.com/brimdata/super/lake"
	"github.com/brimdata/super/lakeparse"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zio/anyio"
	"github.com/segmentio/ksuid"
)

type Source struct {
	engine storage.Engine
	lake   *lake.Root
}

func NewSource(engine storage.Engine, lake *lake.Root) *Source {
	return &Source{
		engine: engine,
		lake:   lake,
	}
}

func (s *Source) IsLake() bool {
	return s.lake != nil
}

func (s *Source) Lake() *lake.Root {
	return s.lake
}

func (s *Source) PoolID(ctx context.Context, name string) (ksuid.KSUID, error) {
	if id, err := lakeparse.ParseID(name); err == nil {
		if _, err := s.lake.OpenPool(ctx, id); err == nil {
			return id, nil
		}
	}
	return s.lake.PoolID(ctx, name)
}

func (s *Source) CommitObject(ctx context.Context, id ksuid.KSUID, name string) (ksuid.KSUID, error) {
	if s.lake != nil {
		return s.lake.CommitObject(ctx, id, name)
	}
	return ksuid.Nil, nil
}

func (s *Source) SortKeys(ctx context.Context, src dag.Op) order.SortKeys {
	if s.lake != nil {
		return s.lake.SortKeys(ctx, src)
	}
	return nil
}

func (s *Source) Open(ctx context.Context, zctx *super.Context, path, format string, pushdown zbuf.Filter, demandOut demand.Demand) (zbuf.Puller, error) {
	if path == "-" {
		path = "stdio:stdin"
	}
	file, err := anyio.Open(ctx, zctx, s.engine, path, demandOut, anyio.ReaderOpts{Format: format})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	scanner, err := zbuf.NewScanner(ctx, file, pushdown)
	if err != nil {
		file.Close()
		return nil, err
	}
	sn := zbuf.NamedScanner(scanner, path)
	return &closePuller{sn, file}, nil
}

func (s *Source) OpenHTTP(ctx context.Context, zctx *super.Context, url, format, method string, headers http.Header, body io.Reader, demandOut demand.Demand) (zbuf.Puller, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header = headers
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	file, err := anyio.NewFile(zctx, resp.Body, url, demandOut, anyio.ReaderOpts{Format: format})
	if err != nil {
		resp.Body.Close()
		return nil, fmt.Errorf("%s: %w", url, err)
	}
	scanner, err := zbuf.NewScanner(ctx, file, nil)
	if err != nil {
		file.Close()
		return nil, err
	}
	return &closePuller{scanner, file}, nil
}

type closePuller struct {
	p zbuf.Puller
	c io.Closer
}

func (c *closePuller) Pull(done bool) (zbuf.Batch, error) {
	batch, err := c.p.Pull(done)
	if batch == nil {
		c.c.Close()
	}
	return batch, err
}
