package compiler

import (
	"fmt"

	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/data"
	"github.com/brimdata/super/lakeparse"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/zio"
)

func NewCompiler() runtime.Compiler {
	return &anyCompiler{}
}

func (i *anyCompiler) NewQuery(rctx *runtime.Context, seq ast.Seq, readers []zio.Reader) (runtime.Query, error) {
	if len(readers) != 1 {
		return nil, fmt.Errorf("NewQuery: Zed program expected %d readers", len(readers))
	}
	job, err := NewJob(rctx, seq, data.NewSource(nil, nil), nil)
	if err != nil {
		return nil, err
	}
	return optimizeAndBuild(job, readers)
}

func (*anyCompiler) NewLakeQuery(rctx *runtime.Context, program ast.Seq, parallelism int, head *lakeparse.Commitish) (runtime.Query, error) {
	panic("NewLakeQuery called on compiler.anyCompiler")
}

func (*anyCompiler) NewLakeDeleteQuery(rctx *runtime.Context, program ast.Seq, head *lakeparse.Commitish) (runtime.DeleteQuery, error) {
	panic("NewLakeDeleteQuery called on compiler.anyCompiler")
}
