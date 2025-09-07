package dev

import (
	"flag"

	"github.com/brimdata/super/cmd/super/root"
	"github.com/brimdata/super/pkg/charm"
)

var Spec = &charm.Spec{
	Name:  "dev",
	Usage: "dev sub-command [arguments...]",
	Short: "run specified development tool",
	Long: `
dev runs a development tool identified by the arguments. With no arguments it
prints the list of known dev tools.`,
	New: New,
}

type Command struct {
	*root.Command
}

func init() {
	root.Super.Add(Spec)
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	return &Command{Command: parent.(*root.Command)}, nil
}

func (c *Command) Run(args []string) error {
	return charm.NoRun(args)
}
