package compile

import (
	"context"
	"errors"
	"flag"
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/cli/dbflags"
	"github.com/brimdata/super/cli/outputflags"
	"github.com/brimdata/super/cli/queryflags"
	"github.com/brimdata/super/compiler"
	"github.com/brimdata/super/compiler/describe"
	"github.com/brimdata/super/db"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/exec"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zfmt"
)

type Shared struct {
	dag         bool
	includes    queryflags.Includes
	optimize    bool
	parallel    int
	query       bool
	OutputFlags outputflags.Flags
}

func (s *Shared) SetFlags(fs *flag.FlagSet) {
	fs.BoolVar(&s.dag, "dag", false, "display output as DAG (implied by -O or -P)")
	fs.Var(&s.includes, "I", "source file containing query text (may be repeated)")
	fs.BoolVar(&s.optimize, "O", false, "display optimized DAG")
	fs.IntVar(&s.parallel, "P", 0, "display parallelized DAG")
	fs.BoolVar(&s.query, "C", false, "display DAG or AST as query text")
	s.OutputFlags.SetFlags(fs)
}

func (s *Shared) Run(ctx context.Context, args []string, dbFlags *dbflags.Flags, desc, extInput bool) error {
	if len(s.includes) == 0 && len(args) == 0 {
		return errors.New("no query specified")
	}
	if len(args) > 1 {
		return errors.New("too many arguments")
	}
	var root *db.Root
	if dbFlags != nil {
		dbAPI, err := dbFlags.Open(ctx)
		if err != nil {
			return err
		}
		root = dbAPI.Root()
	}
	var query string
	if len(args) == 1 {
		query = args[0]
	}
	ast, err := compiler.Parse(query, s.includes...)
	if err != nil {
		return err
	}
	if s.parallel > 0 {
		s.optimize = true
	}
	if s.optimize || desc {
		s.dag = true
	}
	if !s.dag {
		if s.query {
			fmt.Println(zfmt.AST(ast.Parsed()))
			return nil
		}
		return s.writeValue(ctx, ast.Parsed())
	}
	rctx := runtime.DefaultContext()
	env := exec.NewEnvironment(nil, root)
	dag, err := compiler.Analyze(rctx, ast, env, extInput)
	if err != nil {
		return err
	}
	if desc {
		description, err := describe.AnalyzeDAG(ctx, dag, env)
		if err != nil {
			return err
		}
		return s.writeValue(ctx, description)
	}
	if s.optimize {
		dag, err = compiler.Optimize(rctx, dag, env, s.parallel)
		if err != nil {
			return err
		}
	}
	if s.query {
		fmt.Println(zfmt.DAG(dag))
		return nil
	}
	return s.writeValue(ctx, dag)
}

func (s *Shared) writeValue(ctx context.Context, v any) error {
	val, err := sup.MarshalBSUP(v)
	if err != nil {
		return err
	}
	writer, err := s.OutputFlags.Open(ctx, storage.NewLocalEngine())
	if err != nil {
		return err
	}
	err = sio.CopyWithContext(ctx, writer, zbuf.NewArray([]super.Value{val}))
	if closeErr := writer.Close(); err == nil {
		err = closeErr
	}
	return err
}
