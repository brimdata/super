package vacate

import (
	"errors"
	"flag"

	"github.com/brimdata/super/cmd/zed/root"
	"github.com/brimdata/super/pkg/charm"
)

var Cmd = &charm.Spec{
	Name:  "vacate",
	Usage: "vacate [options] commit",
	Short: "compact a pool's commit history by squashing old commit objects",
	Long: `
The vacate command compacts the commit history by squashing all of the commit
objects in the history up to the indicated commit and removing the old commits.
No other commit objects in the pool may point at any of the squashed commits.
In particular, no branch may point to any commit that would be deleted.

The branch history may contain pointers to old commit objects, but any attempt
to access them will fail as the underlying commit history will be no longer available.

DANGER ZONE.
There is no prompting or second chances here so use carefully.
Once the pool's commit history has been squashed and old commits is deleted,
they cannot be recovered.
`,
	New: New,
}

type Command struct {
	*root.Command
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	return &Command{Command: parent.(*root.Command)}, nil
}

func (c *Command) Run(args []string) error {
	return errors.New("issue #2545")
}
