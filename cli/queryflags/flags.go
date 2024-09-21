package queryflags

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"slices"

	"github.com/brimdata/super/compiler"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/data"
	"github.com/brimdata/super/compiler/parser"
	"github.com/brimdata/super/compiler/semantic"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zson"
)

type Flags struct {
	Verbose  bool
	Stats    bool
	Includes Includes
	SQL      bool
}

func (f *Flags) SetFlags(fs *flag.FlagSet) {
	fs.BoolVar(&f.Stats, "s", false, "display search stats on stderr")
	fs.Var(&f.Includes, "I", "source file containing Zed query text (may be used multiple times)")
	fs.BoolVar(&f.SQL, "sql", false, "interpret query text as SuperSQL")
}

func (f *Flags) ParseSourcesAndInputs(src string, paths []string) ([]string, ast.Seq, *parser.SourceSet, bool, error) {
	if len(paths) == 0 && src != "" {
		// Consider a lone argument to be a query if it compiles
		// and appears to start with a from or yield operator.
		// Otherwise, consider it a file.
		query, sset, err := compiler.Parse(f.SQL, src, f.Includes...)
		if err != nil {
			return nil, nil, nil, false, err
		}
		s, err := semantic.Analyze(context.Background(), query, data.NewSource(storage.NewLocalEngine(), nil), nil)
		if err != nil {
			if list, ok := err.(parser.ErrorList); ok {
				list.SetSourceSet(sset)
			}
			return nil, nil, nil, false, err
		}
		//XXX we should simplify this logic, e.g., by inserting a null source
		// if no source is given (this is how sql "select count(*)" works with no from)
		if semantic.HasSource(s) {
			return nil, query, sset, false, nil
		}
		if semantic.StartsWithYield(s) {
			return nil, query, sset, true, nil
		}
		return nil, nil, nil, false, errors.New("no data source found")
	}
	query, sset, err := parser.ParseSuperPipe(f.SQL, f.Includes, src)
	if err != nil {
		return nil, nil, nil, false, err
	}
	return paths, query, sset, false, nil
}

func isURLWithKnownScheme(path string, schemes ...string) bool {
	u, err := url.Parse(path)
	if err != nil {
		return false
	}
	return slices.Contains(schemes, u.Scheme)
}

func (f *Flags) PrintStats(stats zbuf.Progress) {
	if f.Stats {
		out, err := zson.Marshal(stats)
		if err != nil {
			out = fmt.Sprintf("error marshaling stats: %s", err)
		}
		fmt.Fprintln(os.Stderr, out)
	}
}
