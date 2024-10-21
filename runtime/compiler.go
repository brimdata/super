package runtime

import (
	"context"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/parser"
	"github.com/brimdata/super/lakeparse"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zio"
	"github.com/segmentio/ksuid"
)

type Compiler interface {
	NewQuery(*Context, ast.Seq, []zio.Reader) (Query, error)
	NewLakeQuery(*Context, ast.Seq, int, *lakeparse.Commitish) (Query, error)
	NewLakeDeleteQuery(*Context, ast.Seq, *lakeparse.Commitish) (DeleteQuery, error)
	Parse(string, ...string) (ast.Seq, *parser.SourceSet, error)
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

func CompileQuery(ctx context.Context, zctx *super.Context, c Compiler, program ast.Seq, sset *parser.SourceSet, readers []zio.Reader) (Query, error) {
	rctx := NewContext(ctx, zctx)
	q, err := c.NewQuery(rctx, program, readers)
	if err != nil {
		rctx.Cancel()
		if list, ok := err.(parser.ErrorList); ok {
			list.SetSourceSet(sset)
		}
		return nil, err
	}
	return q, nil
}

func CompileLakeQuery(ctx context.Context, zctx *super.Context, c Compiler, program ast.Seq, sset *parser.SourceSet, head *lakeparse.Commitish) (Query, error) {
	rctx := NewContext(ctx, zctx)
	q, err := c.NewLakeQuery(rctx, program, 0, head)
	if err != nil {
		rctx.Cancel()
		if list, ok := err.(parser.ErrorList); ok {
			list.SetSourceSet(sset)
		}
		return nil, err
	}
	return q, nil
}
