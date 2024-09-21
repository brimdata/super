package compile

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	"github.com/brimdata/super/cli/lakeflags"
	"github.com/brimdata/super/cmd/super/dev"
	"github.com/brimdata/super/cmd/super/root"
	"github.com/brimdata/super/compiler"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/compiler/ast/dag"
	"github.com/brimdata/super/compiler/data"
	"github.com/brimdata/super/compiler/describe"
	"github.com/brimdata/super/compiler/parser"
	"github.com/brimdata/super/lake"
	"github.com/brimdata/super/pkg/charm"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/zfmt"
)

var spec = &charm.Spec{
	Name:  "compile",
	Usage: "compile [ options ] zed",
	Short: "inspect Zed language abstract syntax trees and compiler stages",
	Long: `
The "zed dev compile" command parses a Zed expression and prints the resulting abstract syntax
tree as JSON object to standard output.  If you have installed the
shortcuts, "zc" is a short cut for the "zed dev compile" command.

"zed dev compile" is a tool for dev and test,
and is also useful to advanced users for understanding how Zed syntax is
translated into an analytics requests sent to the "zed server" search endpoint.

The -O flag is handy for turning on and off the compiler, which lets you see
how the parsed AST is transformed into a runtime object comprised of the
Zed kernel operators.
`,
	New: New,
}

func init() {
	dev.Spec.Add(spec)
}

type Command struct {
	*root.Command
	//XXX this doesn't work right because flags aren't shown in help...
	// we should have a "super db compile" command and a "super compile" command
	LakeFlags lakeflags.Flags
	pigeon    bool
	proc      bool
	canon     bool
	semantic  bool
	optimize  bool
	describe  bool
	parallel  int
	n         int
	includes  includes
	sql      bool
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*root.Command)}
	c.LakeFlags.SetFlags(f)
	f.BoolVar(&c.pigeon, "pigeon", false, "run pigeon version of peg parser")
	f.BoolVar(&c.proc, "proc", false, "run pigeon version of peg parser and marshal into ast.Op")
	f.BoolVar(&c.semantic, "s", false, "display semantically analyzed AST (implies -proc)")
	f.BoolVar(&c.optimize, "O", false, "display optimized, non-filter AST (implies -proc)")
	f.IntVar(&c.parallel, "P", 0, "display parallelized AST (implies -proc)")
	f.BoolVar(&c.canon, "C", false, "display AST in Zed canonical format (implies -proc)")
	f.BoolVar(&c.describe, "D", false, "display describe summary of Zed query (implies -proc)")
	f.Var(&c.includes, "I", "source file containing Zed query text (may be repeated)")
	f.BoolVar(&c.sql, "sql", false, "interpret query text as SuperSQL")
	return c, nil
}

type includes []string

func (i includes) String() string {
	return strings.Join(i, ",")
}

func (i *includes) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func (c *Command) Run(args []string) error {
	ctx, cleanup, err := c.Init()
	if err != nil {
		return err
	}
	defer cleanup()
	if len(args) == 0 && len(c.includes) == 0 {
		return charm.NeedHelp
	}
	c.n = 0
	if c.pigeon {
		c.n++
	}
	if c.proc {
		c.n++
	}
	if c.semantic {
		c.n++
	}
	if c.describe {
		c.n++
	}
	if c.optimize {
		c.n++
	}
	if c.parallel > 0 {
		c.n++
	}
	if c.n == 0 {
		if c.canon {
			c.proc = true
		} else {
			c.pigeon = true
		}
	}
	src := strings.Join(args, " ")
	var lk *lake.Root
	if c.semantic || c.optimize || c.parallel != 0 || c.describe {
		lakeAPI, err := c.LakeFlags.Open(ctx)
		if err == nil {
			lk = lakeAPI.Root()
		}
	}
	return c.parse(ctx, c.sql, src, lk)
}

func (c *Command) header(msg string) {
	if c.n > 1 {
		bars := strings.Repeat("=", len(msg))
		fmt.Printf("/%s\\\n", bars)
		fmt.Printf("|%s|\n", msg)
		fmt.Printf("\\%s/\n", bars)
	}
}

func (c *Command) parse(ctx context.Context, sql bool, z string, lk *lake.Root) error {
	seq, sset, err := compiler.Parse(sql, z, c.includes...)
	if err != nil {
		return err
	}
	if c.pigeon {
		b, err := json.Marshal(seq)
		if err != nil {
			return err
		}
		c.header("pigeon")
		fmt.Println(normalize(b))
	}
	if c.proc {
		c.header("proc")
		c.writeAST(seq)
	}
	if !c.semantic && !c.optimize && c.parallel == 0 && !c.describe {
		return nil
	}
	runtime, err := compiler.NewJob(runtime.DefaultContext(), seq, data.NewSource(nil, lk), nil)
	if err != nil {
		if list, ok := err.(parser.ErrorList); ok {
			list.SetSourceSet(sset)
		}
		return err
	}
	if c.semantic {
		c.header("semantic")
		c.writeDAG(runtime.Entry())
	}
	if c.describe {
		c.header("describe")
		c.writeDescribe(ctx, runtime.Entry(), lk)
	}
	if c.optimize {
		if err := runtime.Optimize(); err != nil {
			return err
		}
		c.header("optimized")
		c.writeDAG(runtime.Entry())
	}
	if c.parallel > 0 {
		if err := runtime.Optimize(); err != nil {
			return err
		}
		if err := runtime.Parallelize(c.parallel); err != nil {
			return err
		}
		c.header("parallelized")
		c.writeDAG(runtime.Entry())
	}
	return nil
}

func (c *Command) writeAST(seq ast.Seq) {
	s, err := astFmt(seq, c.canon)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(s)
	}
}

func (c *Command) writeDAG(seq dag.Seq) {
	s, err := dagFmt(seq, c.canon)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(s)
	}
}

func (c *Command) writeDescribe(ctx context.Context, seq dag.Seq, lk *lake.Root) {
	info, err := describe.AnalyzeDAG(ctx, seq, data.NewSource(nil, lk), nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	out, err := json.MarshalIndent(info, "", "    ")
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(string(out))
	}
}

func normalize(b []byte) (string, error) {
	var v interface{}
	err := json.Unmarshal(b, &v)
	if err != nil {
		return "", err
	}
	out, err := json.MarshalIndent(v, "", "    ")
	return string(out), err
}

func astFmt(seq ast.Seq, canon bool) (string, error) {
	if canon {
		return zfmt.AST(seq), nil
	}
	seqJSON, err := json.Marshal(seq)
	if err != nil {
		return "", err
	}
	return normalize(seqJSON)
}

func dagFmt(seq dag.Seq, canon bool) (string, error) {
	if canon {
		return zfmt.DAG(seq), nil
	}
	dagJSON, err := json.Marshal(seq)
	if err != nil {
		return "", err
	}
	return normalize(dagJSON)
}
