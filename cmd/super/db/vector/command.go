package vector

import (
	"flag"

	"github.com/brimdata/super/cmd/super/db"
	"github.com/brimdata/super/pkg/charm"
)

var spec = &charm.Spec{
	Name:  "vector",
	Usage: "vector [subcommand]",
	Short: "create and delete vectorized versions of lake data",
	Long: `
The vector subcommands control the creation, management, and deletion
of vectorized data in a Zed lake.
`,
	New: New,
}

func init() {
	spec.Add(add)
	spec.Add(del)
	db.Spec.Add(spec)
}

type Command struct {
	*db.Command
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	return &Command{Command: parent.(*db.Command)}, nil
}

func (c *Command) Run(args []string) error {
	if len(args) == 0 {
		return charm.NeedHelp
	}
	return charm.ErrNoRun
}
