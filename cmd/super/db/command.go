package db

import (
	"flag"

	"github.com/brimdata/super/cli/lakeflags"
	"github.com/brimdata/super/cmd/super/root"
	"github.com/brimdata/super/pkg/charm"
)

var Spec = &charm.Spec{
	Name:  "db",
	Usage: "db <sub-command> [options] [arguments...]",
	Short: "run SuperDB data lake commands",
	Long: `
XXX db is a command-line tool for creating, configuring, ingesting into,
querying, and orchestrating Zed data lakes.`,
	New: New,
}

func init() {
	root.Super.Add(Spec)
}

type Command struct {
	*root.Command
	LakeFlags lakeflags.Flags
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*root.Command)}
	c.LakeFlags.SetFlags(f)
	return c, nil
}

func (c *Command) Run(args []string) error {
	return charm.NoRun(args)
}
