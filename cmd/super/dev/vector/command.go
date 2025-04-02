package vector

import (
	"flag"

	"github.com/brimdata/super/cmd/super/dev"
	"github.com/brimdata/super/pkg/charm"
)

var Spec = &charm.Spec{
	Name:  "vector",
	Usage: "vector sub-command [arguments...]",
	Short: "run specified CSUP vector test",
	Long: `
vector runs various tests of the vector cache and runtime as specified by its sub-command.`,
	New: New,
}

type Command struct {
	*dev.Command
}

func init() {
	dev.Spec.Add(Spec)
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	return &Command{Command: parent.(*dev.Command)}, nil
}

func (c *Command) Run(args []string) error {
	return charm.NoRun(args)
}
