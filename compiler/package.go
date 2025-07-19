package compiler

import (
	"context"
	"errors"
	"fmt"

	"github.com/brimdata/super/compiler/dag"
	"github.com/brimdata/super/compiler/optimizer"
	"github.com/brimdata/super/compiler/parser"
	"github.com/brimdata/super/compiler/rungen"
	"github.com/brimdata/super/compiler/semantic"
	"github.com/brimdata/super/lakeparse"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/exec"
	"github.com/brimdata/super/runtime/sam/op"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zio"
)

func Parse(query string, filenames ...string) (*parser.AST, error) {
	return parser.ParseQuery(query, filenames...)
}

func Analyze(ctx context.Context, ast *parser.AST, env *exec.Environment, extInput bool) (dag.Seq, error) {
	return semantic.Analyze(ctx, ast, env, extInput)
}

func Optimize(ctx context.Context, seq dag.Seq, env *exec.Environment, parallel int) (dag.Seq, error) {
	// Call optimize to possible push down a filter predicate into the
	// rungen.Reader so that the BSUP scanner can do Boyer-Moore.
	o := optimizer.New(ctx, env)
	seq, err := o.Optimize(seq)
	if err != nil {
		return nil, err
	}
	if parallel > 1 {
		// For an internal reader (like a shaper on intake), we don't do
		// any parallelization right now though this could be potentially
		// beneficial depending on where the bottleneck is for a given shaper.
		// See issue #2641.
		seq, err = o.Parallelize(seq, parallel)
		if err != nil {
			return nil, err
		}
		seq, err = o.Vectorize(seq)
		if err != nil {
			return nil, err
		}
	}
	return seq, nil
}

func Build(rctx *runtime.Context, seq dag.Seq, env *exec.Environment, readers []zio.Reader) (map[string]zbuf.Puller, zbuf.Meter, error) {
	b := rungen.NewBuilder(rctx, env)
	outputs, err := b.Build(seq, readers...)
	if err != nil {
		return nil, nil, err
	}
	return outputs, b.Meter(), nil
}

func BuildWithBuilder(rctx *runtime.Context, seq dag.Seq, env *exec.Environment, readers []zio.Reader) (map[string]zbuf.Puller, *rungen.Builder, error) {
	b := rungen.NewBuilder(rctx, env)
	outputs, err := b.Build(seq, readers...)
	if err != nil {
		return nil, nil, err
	}
	return outputs, b, nil
}

func CompileWithAST(rctx *runtime.Context, ast *parser.AST, env *exec.Environment, optimize bool, parallel int, readers []zio.Reader) (*exec.Query, error) {
	dag, err := Analyze(rctx, ast, env, len(readers) > 0)
	if err != nil {
		return nil, err
	}
	if optimize {
		dag, err = Optimize(rctx, dag, env, parallel)
		if err != nil {
			return nil, err
		}
	}
	outputs, meter, err := Build(rctx, dag, env, readers)
	if err != nil {
		return nil, err
	}
	return exec.NewQuery(rctx, bundleOutputs(rctx, outputs), meter), nil
}

func Compile(rctx *runtime.Context, env *exec.Environment, optimize bool, parallel int, readers []zio.Reader, query string, filenames ...string) (*exec.Query, error) {
	ast, err := Parse(query, filenames...)
	if err != nil {
		return nil, err
	}
	return CompileWithAST(rctx, ast, env, optimize, parallel, readers)
}

func bundleOutputs(rctx *runtime.Context, outputs map[string]zbuf.Puller) zbuf.Puller {
	switch len(outputs) {
	case 0:
		return nil
	case 1:
		var puller zbuf.Puller
		for k, p := range outputs {
			puller = op.NewCatcher(op.NewSingle(k, p))
		}
		return puller
	default:
		return op.NewMux(rctx, outputs)
	}
}

func VectorFilterCompile(rctx *runtime.Context, query string, env *exec.Environment, head *lakeparse.Commitish) (zbuf.Puller, error) {
	// Eventually the semantic analyzer + rungen will resolve the pool but
	// for now just do this manually.
	if !env.IsLake() {
		return nil, errors.New("non-lake vectorized search not supported")
	}
	poolID, err := env.PoolID(rctx.Context, head.Pool)
	if err != nil {
		return nil, err
	}
	commitID, err := env.CommitObject(rctx.Context, poolID, head.Branch)
	if err != nil {
		return nil, err
	}
	spec, err := head.FromSpec("")
	if err != nil {
		return nil, err
	}
	ast, err := parser.ParseQuery(fmt.Sprintf("%s | %s", spec, query))
	if err != nil {
		return nil, err
	}
	entry, err := semantic.Analyze(rctx.Context, ast, env, false)
	if err != nil {
		return nil, err
	}
	// from -> filter -> output
	if len(entry) != 3 {
		return nil, errors.New("filter query must have a single op")
	}
	f, ok := entry[1].(*dag.Filter)
	if !ok {
		return nil, errors.New("filter query must be a single filter op")
	}
	return rungen.NewBuilder(rctx, env).BuildVamToSeqFilter(f.Expr, poolID, commitID)
}

// XXX currently used only by aggregate test, need to deprecate
func CompileWithSortKey(rctx *runtime.Context, ast *parser.AST, r zio.Reader, sortKey order.SortKey) (*exec.Query, error) {
	env := exec.NewEnvironment(nil, nil)
	seq, err := Analyze(rctx, ast, env, true)
	if err != nil {
		return nil, err
	}
	scan, ok := seq[0].(*dag.DefaultScan)
	if !ok {
		return nil, errors.New("CompileWithSortKey: expected a reader")
	}
	scan.SortKeys = order.SortKeys{sortKey}
	seq, err = Optimize(rctx, seq, env, 0)
	if err != nil {
		return nil, err
	}
	outputs, meter, err := Build(rctx, seq, env, []zio.Reader{r})
	if err != nil {
		return nil, err
	}
	return exec.NewQuery(rctx, bundleOutputs(rctx, outputs), meter), nil
}
