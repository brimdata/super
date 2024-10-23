package compiler

import (
	"errors"
	"fmt"

	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/data"
	"github.com/brimdata/super/lakeparse"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/exec"
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
	scan.SortKeys = order.SortKeys{sortKey}
	return optimizeAndBuild(job, []zio.Reader{r})
}

func (*anyCompiler) NewLakeQuery(rctx *runtime.Context, program ast.Seq, parallelism int, head *lakeparse.Commitish) (runtime.Query, error) {
	panic("NewLakeQuery called on compiler.anyCompiler")
}

func (*anyCompiler) NewLakeDeleteQuery(rctx *runtime.Context, program ast.Seq, head *lakeparse.Commitish) (runtime.DeleteQuery, error) {
	panic("NewLakeDeleteQuery called on compiler.anyCompiler")
}
