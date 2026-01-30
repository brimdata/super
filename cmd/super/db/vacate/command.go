package vacate

import (
	"errors"
	"flag"

	"github.com/brimdata/super/cmd/super/db"
	"github.com/brimdata/super/pkg/charm"
)

var spec = &charm.Spec{
	Name:  "vacate",
	Usage: "vacate [options] commit",
	Short: "compact a pool's commit history by squashing old commit objects",
	Long: `
See https://superdb.org/command/db.html#super-db-vacate
`,
	New: New,
}

func init() {
	db.Spec.Add(spec)
}

type Command struct {
	*db.Command
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	return &Command{Command: parent.(*db.Command)}, nil
}

func (c *Command) Run(args []string) error {
	return errors.New("issue #2545")
}
