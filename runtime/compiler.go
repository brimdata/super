package runtime

import (
	"context"
	"io"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/compiler/ast"
	"github.com/brimdata/zed/lakeparse"
	"github.com/brimdata/zed/zbuf"
	"github.com/brimdata/zed/zio"
	"github.com/segmentio/ksuid"
)

type Compiler interface {
	NewQuery(*Context, ast.Seq, []zio.Reader) (Query, error)
	NewLakeQuery(*Context, ast.Seq, int, *lakeparse.Commitish) (Query, error)
	NewLakeDeleteQuery(*Context, ast.Seq, *lakeparse.Commitish) (DeleteQuery, error)
	Parse(string, ...string) (ast.Seq, error)
}

type Query interface {
	zbuf.Puller
	io.Closer
	Progress() zbuf.Progress
	Meter() zbuf.Meter
}

type DeleteQuery interface {
	Query
	DeletionSet() []ksuid.KSUID
}

func AsReader(q Query) zio.Reader {
	return zbuf.PullerReader(q)
}

func AsProgressReadCloser(q Query) zbuf.ProgressReadCloser {
	return struct {
		zio.Reader
		io.Closer
		zbuf.Meter
	}{AsReader(q), q, q}
}

func CompileQuery(ctx context.Context, zctx *zed.Context, c Compiler, program ast.Seq, readers []zio.Reader) (Query, error) {
	rctx := NewContext(ctx, zctx)
	q, err := c.NewQuery(rctx, program, readers)
	if err != nil {
		rctx.Cancel()
		return nil, err
	}
	return q, nil
}

func CompileLakeQuery(ctx context.Context, zctx *zed.Context, c Compiler, program ast.Seq, head *lakeparse.Commitish) (Query, error) {
	rctx := NewContext(ctx, zctx)
	q, err := c.NewLakeQuery(rctx, program, 0, head)
	if err != nil {
		rctx.Cancel()
		return nil, err
	}
	return q, nil
}
