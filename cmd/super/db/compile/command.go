package compile

import (
	"flag"

	"github.com/brimdata/super/cmd/super/compile"
	"github.com/brimdata/super/cmd/super/db"
	"github.com/brimdata/super/pkg/charm"
)

var spec = &charm.Spec{
	Name:  "compile",
	Usage: "compile [ options ] spq|sql",
	Short: "compile a lake query for inspection and debugging",
	Long: `
The "super db compile" command is just like the "super compile" command except 
it compiles the query for a SuperDB data lake instead of a local file system.
The primary difference here is that "from" operators on a lake work with data
stored in the lake whereas "from" operators on file system work with local files.
In both cases, "from" can also retrieve data from HTTP APIs via URL.

See the "super compile" command help for futher information.
`,
	New: New,
}

func init() {
	db.Spec.Add(spec)
}

type Command struct {
	parent   *db.Command
	shared   compile.Shared
	describe bool
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{parent: parent.(*db.Command)}
	c.shared.SetFlags(f)
	f.BoolVar(&c.describe, "describe", false, "emit describe endpoint results for this query")
	return c, nil
}

func (c *Command) Run(args []string) error {
	ctx, cleanup, err := c.parent.Init(&c.shared.OutputFlags)
	if err != nil {
		return err
	}
	defer cleanup()
	return c.shared.Run(ctx, args, &c.parent.LakeFlags, c.describe)
}
