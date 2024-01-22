package compiler

import (
	"errors"
	"fmt"

	"github.com/brimdata/zed/compiler/ast"
	"github.com/brimdata/zed/compiler/data"
	"github.com/brimdata/zed/lakeparse"
	"github.com/brimdata/zed/order"
	"github.com/brimdata/zed/runtime"
	"github.com/brimdata/zed/runtime/exec"
	"github.com/brimdata/zed/zio"
)

func NewCompiler() runtime.Compiler {
	return &anyCompiler{}
}

func (i *anyCompiler) NewQuery(rctx *runtime.Context, seq ast.Seq, readers []zio.Reader) (runtime.Query, error) {
	if len(readers) != 1 {
		return nil, fmt.Errorf("NewQuery: Zed program expected %d readers", len(readers))
	}
	return CompileWithSortKey(rctx, seq, readers[0], order.SortKey{})
}

// XXX currently used only by group-by test, need to deprecate
func CompileWithSortKey(rctx *runtime.Context, seq ast.Seq, r zio.Reader, sortKey order.SortKey) (*exec.Query, error) {
	job, err := NewJob(rctx, seq, data.NewSource(nil, nil), nil)
	if err != nil {
		return nil, err
	}
	scan, ok := job.DefaultScan()
	if !ok {
		return nil, errors.New("CompileWithSortKey: Zed program expected a reader")
	}
	scan.SortKey = sortKey
	return optimizeAndBuild(job, []zio.Reader{r})
}

func (*anyCompiler) NewLakeQuery(rctx *runtime.Context, program ast.Seq, parallelism int, head *lakeparse.Commitish) (runtime.Query, error) {
	panic("NewLakeQuery called on compiler.anyCompiler")
}

func (*anyCompiler) NewLakeDeleteQuery(rctx *runtime.Context, program ast.Seq, head *lakeparse.Commitish) (runtime.DeleteQuery, error) {
	panic("NewLakeDeleteQuery called on compiler.anyCompiler")
}
