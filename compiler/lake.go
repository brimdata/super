package compiler

import (
	"errors"
	goruntime "runtime"

	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/compiler/data"
	"github.com/brimdata/super/compiler/kernel"
	"github.com/brimdata/super/compiler/optimizer"
	"github.com/brimdata/super/compiler/parser"
	"github.com/brimdata/super/compiler/semantic"
	"github.com/brimdata/super/lake"
	"github.com/brimdata/super/lakeparse"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/exec"
)

var Parallelism = goruntime.GOMAXPROCS(0) //XXX

type lakeCompiler struct {
	anyCompiler
	src *data.Source
}

func NewLakeCompiler(r *lake.Root) runtime.Compiler {
	// We configure a remote storage engine into the lake compiler so that
	// "from" operators that source http or s3 will work, but stdio and
	// file system accesses will be rejected at open time.
	return &lakeCompiler{src: data.NewSource(storage.NewRemoteEngine(), r)}
}

func (l *lakeCompiler) NewLakeQuery(rctx *runtime.Context, ast *parser.AST, parallelism int) (runtime.Query, error) {
	job, err := NewJob(rctx, ast, l.src, false)
	if err != nil {
		return nil, err
	}
	if _, ok := job.DefaultScan(); ok {
		return nil, errors.New("query must include a 'from' operator")
	}
	if err := job.Optimize(); err != nil {
		return nil, err
	}
	if parallelism == 0 {
		parallelism = Parallelism
	}
	if parallelism > 1 {
		if err := job.Parallelize(parallelism); err != nil {
			return nil, err
		}
	}
	if err := job.Build(); err != nil {
		return nil, err
	}
	return exec.NewQuery(job.rctx, job.Puller(), job.builder.Meter()), nil
}

func (l *lakeCompiler) NewLakeDeleteQuery(rctx *runtime.Context, ast *parser.AST, head *lakeparse.Commitish) (runtime.DeleteQuery, error) {
	job, err := newDeleteJob(rctx, ast, head.Pool, head.Branch, l.src)
	if err != nil {
		return nil, err
	}
	if err := job.OptimizeDeleter(Parallelism); err != nil {
		return nil, err
	}
	if err := job.Build(); err != nil {
		return nil, err
	}
	return exec.NewDeleteQuery(rctx, job.Puller(), job.builder.Deletes()), nil
}

type InvalidDeleteWhereQuery struct{}

func (InvalidDeleteWhereQuery) Error() string {
	return "invalid delete where query: must be a single filter operation"
}

func newDeleteJob(rctx *runtime.Context, parsed *parser.AST, pool, branch string, src *data.Source) (*Job, error) {
	if err := parsed.ConvertToDeleteWhere(pool, branch); err != nil {
		return nil, err
	}
	seq := parsed.Parsed()
	if len(seq) != 2 {
		return nil, &InvalidDeleteWhereQuery{}
	}
	entry, err := semantic.Analyze(rctx.Context, parsed, src, false)
	if err != nil {
		return nil, err
	}
	if _, ok := entry[1].(*dag.Filter); !ok {
		return nil, &InvalidDeleteWhereQuery{}
	}
	return &Job{
		rctx:      rctx,
		builder:   kernel.NewBuilder(rctx, src),
		optimizer: optimizer.New(rctx.Context, src),
		entry:     entry,
	}, nil
}
