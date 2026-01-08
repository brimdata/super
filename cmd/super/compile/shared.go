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
	"github.com/brimdata/super/compiler/parser"
	"github.com/brimdata/super/compiler/sfmt"
	"github.com/brimdata/super/compiler/srcfiles"
	"github.com/brimdata/super/db"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/exec"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sup"
)

type Shared struct {
	dag         bool
	dynamic     bool
	optimize    bool
	parallel    int
	query       bool
	queryFlags  queryflags.QueryTextFlags
	OutputFlags outputflags.Flags
}

func (s *Shared) SetFlags(fs *flag.FlagSet) {
	fs.BoolVar(&s.dag, "dag", false, "display output as DAG (implied by -O or -P)")
	fs.BoolVar(&s.dynamic, "dynamic", false, "disable static type checking of inputs on DAG")
	fs.BoolVar(&s.optimize, "O", false, "display optimized DAG")
	fs.IntVar(&s.parallel, "P", 0, "display parallelized DAG")
	fs.BoolVar(&s.query, "C", false, "display DAG or AST as query text")
	s.OutputFlags.SetFlags(fs)
	s.queryFlags.SetFlags(fs)
}

func (s *Shared) Run(ctx context.Context, args []string, dbFlags *dbflags.Flags, desc bool) error {
	if len(s.queryFlags.Query) == 0 && len(args) == 0 {
		return errors.New("no query specified")
	}
	var inputs []string
	if len(args) > 0 {
		s.queryFlags.Query = append(s.queryFlags.Query, &srcfiles.PlainInput{Text: args[0]})
		inputs = args[1:]
	}
	var root *db.Root
	if dbFlags != nil {
		dbAPI, err := dbFlags.Open(ctx)
		if err != nil {
			return err
		}
		root = dbAPI.Root()
	}
	ast, err := parser.ParseFiles(s.queryFlags.Query)
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
			fmt.Println(sfmt.AST(ast.Parsed()))
			return nil
		}
		return s.writeValue(ctx, ast.Parsed())
	}
	if len(inputs) > 0 {
		ast.PrependFileScan(inputs)
	}
	rctx := runtime.DefaultContext()
	env := exec.NewEnvironment(storage.NewLocalEngine(), root)
	env.Dynamic = s.dynamic
	dag, err := compiler.Analyze(rctx, ast, env, false)
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
		if err := compiler.Optimize(rctx, dag, env, s.parallel); err != nil {
			return err
		}
	}
	if s.query {
		fmt.Println(sfmt.DAG(dag))
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
	err = sio.CopyWithContext(ctx, writer, sbuf.NewArray([]super.Value{val}))
	if closeErr := writer.Close(); err == nil {
		err = closeErr
	}
	return err
}
