package runtime

import (
	"context"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/parser"
	"github.com/brimdata/super/dbid"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/zbuf"
	"github.com/segmentio/ksuid"
)

type Compiler interface {
	NewQuery(*Context, *parser.AST, []sio.Reader, int) (Query, error)
	NewDeleteQuery(*Context, *parser.AST, *dbid.Commitish) (DeleteQuery, error)
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

func CompileQuery(ctx context.Context, sctx *super.Context, c Compiler, ast *parser.AST, readers []sio.Reader) (Query, error) {
	rctx := NewContext(ctx, sctx)
	q, err := c.NewQuery(rctx, ast, readers, 0)
	if err != nil {
		rctx.Cancel()
		return nil, err
	}
	return q, nil
}

func CompileQueryForDB(ctx context.Context, sctx *super.Context, c Compiler, ast *parser.AST) (Query, error) {
	rctx := NewContext(ctx, sctx)
	q, err := c.NewQuery(rctx, ast, nil, 0)
	if err != nil {
		rctx.Cancel()
		return nil, err
	}
	return q, nil
}
