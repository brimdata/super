package compile

import (
	"flag"

	"github.com/brimdata/super/cmd/super/root"
	"github.com/brimdata/super/pkg/charm"
)

var spec = &charm.Spec{
	Name:  "compile",
	Usage: "compile [ options ] query",
	Short: "compile a SuperSQL query for inspection and debugging",
	Long: `
This command parses a query and emits the resulting abstract syntax
tree (AST) or runtime directed acyclic graph (DAG) in the output format desired.
Use "-dag" to specify the DAG form; otherwise, the AST form is assumed.

The "-C" option causes the output to be shown as query language source
instead of the AST.  This is particularly helpful to see how SuperSQL queries
in their abbreviated form are translated into the exanded, pedantic form.
The DAG can also be formatted as query-style text
but the resulting text is informational only and does not conform to
any query syntax.  When "-C" is specified, the result is sent to stdout
and the "-f" and "-o" options have no effect.

This command is often used for dev and test but
is also useful to advanced users for understanding how SuperSQL syntax is
parsed into an AST or compiled into a runtime DAG.
`,
	New: New,
}

func init() {
	root.Super.Add(spec)
}

type Command struct {
	*root.Command
	shared Shared
	files  bool
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*root.Command)}
	f.BoolVar(&c.files, "files", false, "compile query as if command-line input files are present)")
	c.shared.SetFlags(f)
	return c, nil
}

func (c *Command) Run(args []string) error {
	ctx, cleanup, err := c.Init(&c.shared.OutputFlags)
	if err != nil {
		return err
	}
	defer cleanup()
	return c.shared.Run(ctx, args, nil, false, c.files)
}
