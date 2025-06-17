package compiler

import (
	goruntime "runtime"

	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/compiler/optimizer"
	"github.com/brimdata/super/compiler/parser"
	"github.com/brimdata/super/lake"
	"github.com/brimdata/super/lakeparse"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/exec"
	"github.com/brimdata/super/zio"
)

var Parallelism = goruntime.GOMAXPROCS(0) //XXX

type compiler struct {
	env *exec.Environment
}

func NewCompiler(local storage.Engine) runtime.Compiler {
	return NewCompilerWithEnv(exec.NewEnvironment(local, nil))
}

func NewLakeCompiler(lk *lake.Root) runtime.Compiler {
	// We configure a remote storage engine into the lake compiler so that
	// "from" operators that source http or s3 will work, but stdio and
	// file system accesses will be rejected at open time.
	return NewCompilerWithEnv(exec.NewEnvironment(storage.NewRemoteEngine(), lk))
}

func NewCompilerWithEnv(env *exec.Environment) runtime.Compiler {
	return &compiler{env}
}

func (c *compiler) NewQuery(rctx *runtime.Context, ast *parser.AST, readers []zio.Reader, parallelism int) (runtime.Query, error) {
	if parallelism == 0 {
		parallelism = Parallelism
	}
	return CompileWithAST(rctx, ast, c.env, true, parallelism, readers)
}

func (l *compiler) NewLakeDeleteQuery(rctx *runtime.Context, ast *parser.AST, head *lakeparse.Commitish) (runtime.DeleteQuery, error) {
	if err := ast.ConvertToDeleteWhere(head.Pool, head.Branch); err != nil {
		return nil, err
	}
	seq := ast.Parsed()
	if len(seq) != 2 {
		return nil, &InvalidDeleteWhereQuery{}
	}
	dagSeq, err := Analyze(rctx, ast, l.env, false)
	if err != nil {
		return nil, err
	}
	if _, ok := dagSeq[1].(*dag.Filter); !ok {
		return nil, &InvalidDeleteWhereQuery{}
	}
	dagSeq, err = optimizer.New(rctx, l.env).OptimizeDeleter(dagSeq, Parallelism)
	if err != nil {
		return nil, err
	}
	outputs, b, err := BuildWithBuilder(rctx, dagSeq, l.env, nil)
	if err != nil {
		return nil, err
	}
	return exec.NewDeleteQuery(rctx, bundleOutputs(rctx, outputs), b.Deletes()), nil
}

type InvalidDeleteWhereQuery struct{}

func (InvalidDeleteWhereQuery) Error() string {
	return "invalid delete where query: must be a single filter operation"
}
