package exec

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/db"
	"github.com/brimdata/super/dbid"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/runtime/vam"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sio/anyio"
	"github.com/brimdata/super/sio/csupio"
	"github.com/brimdata/super/sio/parquetio"
	"github.com/brimdata/super/vector"
	"github.com/segmentio/ksuid"
)

type Environment struct {
	engine storage.Engine
	db     *db.Root
	useVAM bool
}

func NewEnvironment(engine storage.Engine, d *db.Root) *Environment {
	return &Environment{
		engine: engine,
		db:     d,
		useVAM: os.Getenv("SUPER_VAM") != "",
	}
}

func (e *Environment) Engine() storage.Engine {
	return e.engine
}

func (e *Environment) UseVAM() bool {
	return e.useVAM
}

func (e *Environment) SetUseVAM() {
	e.useVAM = true
}

func (e *Environment) IsAttached() bool {
	return e.db != nil
}

func (e *Environment) DB() *db.Root {
	return e.db
}

func (e *Environment) PoolID(ctx context.Context, name string) (ksuid.KSUID, error) {
	if id, err := dbid.ParseID(name); err == nil {
		if _, err := e.db.OpenPool(ctx, id); err == nil {
			return id, nil
		}
	}
	return e.db.PoolID(ctx, name)
}

func (e *Environment) CommitObject(ctx context.Context, id ksuid.KSUID, name string) (ksuid.KSUID, error) {
	if e.db != nil {
		return e.db.CommitObject(ctx, id, name)
	}
	return ksuid.Nil, nil
}

func (e *Environment) SortKeys(ctx context.Context, src dag.Op) order.SortKeys {
	if e.db != nil {
		return e.db.SortKeys(ctx, src)
	}
	return nil
}

func (e *Environment) Open(ctx context.Context, sctx *super.Context, path, format string, pushdown sbuf.Pushdown) (sbuf.Puller, error) {
	if path == "-" {
		path = "stdio:stdin"
	}
	var fields []field.Path
	if pushdown != nil {
		if proj := pushdown.Projection(); proj != nil {
			fields = proj.Paths()
		}
	}
	file, err := anyio.Open(ctx, sctx, e.engine, path, anyio.ReaderOpts{Fields: fields, Format: format})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	scanner, err := sbuf.NewScanner(ctx, file, pushdown)
	if err != nil {
		file.Close()
		return nil, err
	}
	sn := sbuf.NamedScanner(scanner, path)
	return &closePuller{sn, file}, nil
}

func (*Environment) OpenHTTP(ctx context.Context, sctx *super.Context, url, format, method string, headers http.Header, body io.Reader, fields []field.Path) (sbuf.Puller, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header = headers
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	file, err := anyio.NewFile(sctx, resp.Body, url, anyio.ReaderOpts{Fields: fields, Format: format})
	if err != nil {
		resp.Body.Close()
		return nil, fmt.Errorf("%s: %w", url, err)
	}
	scanner, err := sbuf.NewScanner(ctx, file, nil)
	if err != nil {
		file.Close()
		return nil, err
	}
	return &closePuller{scanner, file}, nil
}

type closePuller struct {
	p sbuf.Puller
	c io.Closer
}

func (c *closePuller) Pull(done bool) (sbuf.Batch, error) {
	batch, err := c.p.Pull(done)
	if batch == nil {
		c.c.Close()
	}
	return batch, err
}

func (e *Environment) VectorOpen(ctx context.Context, sctx *super.Context, path, format string, pushdown sbuf.Pushdown) (vector.Puller, error) {
	if path == "-" {
		path = "stdio:stdin"
	}
	uri, err := storage.ParseURI(path)
	if err != nil {
		return nil, err
	}
	r, err := e.engine.Get(ctx, uri)
	if err != nil {
		return nil, err
	}
	var puller vector.Puller
	switch format {
	case "csup":
		puller, err = csupio.NewVectorReader(ctx, sctx, r, pushdown)
	case "parquet":
		puller, err = parquetio.NewVectorReader(ctx, sctx, r, pushdown)
	default:
		var sbufPuller sbuf.Puller
		sbufPuller, err = e.Open(ctx, sctx, path, format, nil)
		puller = vam.NewDematerializer(sbufPuller)
	}
	if err != nil {
		r.Close()
		return nil, err
	}
	return puller, nil
}
