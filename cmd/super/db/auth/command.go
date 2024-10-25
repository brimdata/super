package auth

import (
	"flag"

	"github.com/brimdata/super/cmd/super/db"
	"github.com/brimdata/super/pkg/charm"
)

var spec = &charm.Spec{
	Name:  "auth",
	Usage: "auth [subcommand]",
	Short: "authentication and authorization commands",
	Long:  ``,
	New:   New,
}

func init() {
	spec.Add(Login)
	spec.Add(Logout)
	spec.Add(Method)
	spec.Add(Store)
	spec.Add(Verify)
	db.Spec.Add(spec)
}

type Command struct {
	*db.Command
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	return &Command{Command: parent.(*db.Command)}, nil
}

func (c *Command) Run(args []string) error {
	return charm.NoRun(args)
}
