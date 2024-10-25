package compile

import (
	"context"
	"errors"
	"flag"
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/cli/lakeflags"
	"github.com/brimdata/super/cli/outputflags"
	"github.com/brimdata/super/cli/queryflags"
	"github.com/brimdata/super/compiler"
	"github.com/brimdata/super/compiler/data"
	"github.com/brimdata/super/compiler/describe"
	"github.com/brimdata/super/compiler/parser"
	"github.com/brimdata/super/lake"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zfmt"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zson"
)

type Shared struct {
	dag         bool
	includes    queryflags.Includes
	optimize    bool
	parallel    int
	query       bool
	sql         bool
	OutputFlags outputflags.Flags
}

func (s *Shared) SetFlags(fs *flag.FlagSet) {
	fs.BoolVar(&s.dag, "dag", false, "display output as DAG (implied by -O or -P)")
	fs.Var(&s.includes, "I", "source file containing query text (may be repeated)")
	fs.BoolVar(&s.optimize, "O", false, "display optimized DAG")
	fs.IntVar(&s.parallel, "P", 0, "display parallelized DAG")
	fs.BoolVar(&s.query, "C", false, "display DAG or AST as query text")
	fs.BoolVar(&s.sql, "sql", false, "force a strict SQL intepretation of the query text")
	s.OutputFlags.SetFlags(fs)
}

func (s *Shared) Run(ctx context.Context, args []string, lakeFlags *lakeflags.Flags, desc bool) error {
	if len(s.includes) == 0 && len(args) == 0 {
		return errors.New("no query specified")
	}
	if len(args) > 1 {
		return errors.New("too many arguments")
	}
	var lk *lake.Root
	if lakeFlags != nil {
		lakeAPI, err := lakeFlags.Open(ctx)
		if err != nil {
			return err
		}
		lk = lakeAPI.Root()
	}
	var query string
	if len(args) == 1 {
		query = args[0]
	}
	seq, sset, err := compiler.Parse(query, s.includes...)
	if err != nil {
		return err
	}
	if s.optimize || s.parallel > 0 || desc {
		s.dag = true
	}
	if !s.dag {
		if s.query {
			fmt.Println(zfmt.AST(seq))
			return nil
		}
		return s.writeValue(ctx, seq)
	}
	runtime, err := compiler.NewJob(runtime.DefaultContext(), seq, data.NewSource(nil, lk), nil)
	if err != nil {
		if list, ok := err.(parser.ErrorList); ok {
			list.SetSourceSet(sset)
		}
		return err
	}
	if desc {
		description, err := describe.AnalyzeDAG(ctx, runtime.Entry(), data.NewSource(nil, lk), nil)
		if err != nil {
			return err
		}
		return s.writeValue(ctx, description)
	}
	if s.parallel > 0 {
		if err := runtime.Optimize(); err != nil {
			return err
		}
		if err := runtime.Parallelize(s.parallel); err != nil {
			return err
		}
	} else if s.optimize {
		if err := runtime.Optimize(); err != nil {
			return err
		}
	}
	if s.query {
		fmt.Println(zfmt.DAG(runtime.Entry()))
		return nil
	}
	return s.writeValue(ctx, runtime.Entry())
}

func (s *Shared) writeValue(ctx context.Context, v any) error {
	val, err := zson.MarshalZNG(v)
	if err != nil {
		return err
	}
	writer, err := s.OutputFlags.Open(ctx, storage.NewLocalEngine())
	if err != nil {
		return err
	}
	err = zio.CopyWithContext(ctx, writer, zbuf.NewArray([]super.Value{val}))
	if closeErr := writer.Close(); err == nil {
		err = closeErr
	}
	return err
}
