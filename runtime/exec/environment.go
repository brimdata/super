package exec

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/lake"
	"github.com/brimdata/super/lakeparse"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zio/anyio"
	"github.com/segmentio/ksuid"
)

type Environment struct {
	engine storage.Engine
	lake   *lake.Root
}

func NewEnvironment(engine storage.Engine, lake *lake.Root) *Environment {
	return &Environment{
		engine: engine,
		lake:   lake,
	}
}

func (e *Environment) IsLake() bool {
	return e.lake != nil
}

func (e *Environment) Lake() *lake.Root {
	return e.lake
}

func (e *Environment) PoolID(ctx context.Context, name string) (ksuid.KSUID, error) {
	if id, err := lakeparse.ParseID(name); err == nil {
		if _, err := e.lake.OpenPool(ctx, id); err == nil {
			return id, nil
		}
	}
	return e.lake.PoolID(ctx, name)
}

func (e *Environment) CommitObject(ctx context.Context, id ksuid.KSUID, name string) (ksuid.KSUID, error) {
	if e.lake != nil {
		return e.lake.CommitObject(ctx, id, name)
	}
	return ksuid.Nil, nil
}

func (e *Environment) SortKeys(ctx context.Context, src dag.Op) order.SortKeys {
	if e.lake != nil {
		return e.lake.SortKeys(ctx, src)
	}
	return nil
}

func (e *Environment) Open(ctx context.Context, zctx *super.Context, path, format string, fields []field.Path, pushdown zbuf.Filter) (zbuf.Puller, error) {
	if path == "-" {
		path = "stdio:stdin"
	}
	file, err := anyio.Open(ctx, zctx, e.engine, path, anyio.ReaderOpts{Fields: fields, Format: format})
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

func (*Environment) OpenHTTP(ctx context.Context, zctx *super.Context, url, format, method string, headers http.Header, body io.Reader, fields []field.Path) (zbuf.Puller, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header = headers
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	file, err := anyio.NewFile(zctx, resp.Body, url, anyio.ReaderOpts{Fields: fields, Format: format})
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
