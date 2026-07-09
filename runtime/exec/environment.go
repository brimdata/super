package exec

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/db"
	"github.com/brimdata/super/dbid"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sio/anyio"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/vio"
	"github.com/segmentio/ksuid"
)

type ConcurrentPuller interface {
	vio.Puller
	ConcurrentPull(done bool, id int) (vector.Any, error)
}

type Environment struct {
	engine storage.Engine
	db     *db.Root

	Dynamic          bool
	IgnoreOpenErrors bool
	ReaderOpts       anyio.ReaderOpts
	SampleSize       int
	Stdin            vio.Puller
}

func NewEnvironment(engine storage.Engine, d *db.Root) *Environment {
	return &Environment{
		engine: engine,
		db:     d,
	}
}

func (e *Environment) Engine() storage.Engine {
	return e.engine
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

func (e *Environment) Open(ctx context.Context, sctx *super.Context, path, format string, p sbuf.Pushdown, concurrentReaders int) (ConcurrentPuller, error) {
	if path == "-" {
		path = "stdio:stdin"
	}
	if path == "stdio:stdin" && e.Stdin != nil {
		return newConcurrentPuller(path, e.Stdin), nil
	}
	file, err := anyio.Open(ctx, sctx, e.engine, path, e.readerOpts(p, format, concurrentReaders))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return newConcurrentPuller(path, file.Puller), nil
}

func (e *Environment) readerOpts(p sbuf.Pushdown, format string, concurrentReaders int) anyio.ReaderOpts {
	o := e.ReaderOpts
	o.Pushdown = p
	o.ConcurrentReaders = concurrentReaders
	if format != "" {
		o.Format = format
	}
	return o
}

func (e *Environment) OpenHTTP(ctx context.Context, sctx *super.Context, url, format, method string, headers http.Header, body io.Reader, p sbuf.Pushdown) (vio.Puller, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header = headers
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	file, err := anyio.NewFile(ctx, sctx, resp.Body, url, e.readerOpts(p, format, 1))
	if err != nil {
		resp.Body.Close()
		return nil, fmt.Errorf("%s: %w", url, err)
	}
	return file, nil
}

func newConcurrentPuller(path string, puller vio.Puller) ConcurrentPuller {
	cp, ok := puller.(ConcurrentPuller)
	if !ok {
		cp = &concurrentPuller{puller}
	}
	return &errorPrefixConcurrentPuller{cp, path}
}

type concurrentPuller struct {
	vio.Puller
}

func (c *concurrentPuller) ConcurrentPull(done bool, id int) (vector.Any, error) {
	return c.Pull(done)
}

type errorPrefixConcurrentPuller struct {
	ConcurrentPuller
	prefix string
}

func (e *errorPrefixConcurrentPuller) ConcurrentPull(done bool, id int) (vector.Any, error) {
	vec, err := e.ConcurrentPuller.ConcurrentPull(done, id)
	if err != nil {
		err = fmt.Errorf("%s: %w", e.prefix, err)
	}
	return vec, err
}

func (e *errorPrefixConcurrentPuller) Pull(done bool) (vector.Any, error) {
	vec, err := e.ConcurrentPuller.Pull(done)
	if err != nil {
		err = fmt.Errorf("%s: %w", e.prefix, err)
	}
	return vec, err
}
