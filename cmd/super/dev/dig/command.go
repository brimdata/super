package dig

import (
	"flag"

	"github.com/brimdata/super/cmd/super/dev"
	"github.com/brimdata/super/pkg/charm"
)

var Spec = &charm.Spec{
	Name:  "dig",
	Usage: "dig sub-command [arguments...]",
	Short: "extract useful information from Zed streams or files",
	Long: `
The dig command provide various debug and test functions regarding the Zed family
of formats. When run with no arguments or -h, it lists help for the dig sub-commands.`,
	New: New,
}

func init() {
	dev.Spec.Add(Spec)
}

type Command struct {
	*dev.Command
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	return &Command{Command: parent.(*dev.Command)}, nil
}

func (c *Command) Run(args []string) error {
	return charm.NoRun(args)
}
