package runtime

import (
	"context"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/parser"
	"github.com/brimdata/super/lakeparse"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zio"
	"github.com/segmentio/ksuid"
)

type Compiler interface {
	NewQuery(*Context, *parser.AST, []zio.Reader, int) (Query, error)
	NewLakeDeleteQuery(*Context, *parser.AST, *lakeparse.Commitish) (DeleteQuery, error)
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

func CompileQuery(ctx context.Context, zctx *super.Context, c Compiler, ast *parser.AST, readers []zio.Reader) (Query, error) {
	rctx := NewContext(ctx, zctx)
	q, err := c.NewQuery(rctx, ast, readers, 0)
	if err != nil {
		rctx.Cancel()
		return nil, err
	}
	return q, nil
}

func CompileLakeQuery(ctx context.Context, zctx *super.Context, c Compiler, ast *parser.AST) (Query, error) {
	rctx := NewContext(ctx, zctx)
	q, err := c.NewQuery(rctx, ast, nil, 0)
	if err != nil {
		rctx.Cancel()
		return nil, err
	}
	return q, nil
}
