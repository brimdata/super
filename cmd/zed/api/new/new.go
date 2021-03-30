package new

import (
	"errors"
	"flag"
	"fmt"

	apicmd "github.com/brimdata/zed/cmd/zed/api"
	"github.com/brimdata/zed/pkg/charm"
)

var NewSpec = &charm.Spec{
	Name:  "new",
	Usage: "new [spacename]",
	Short: "create a new space",
	Long: `The new command takes a single argument and creates a new, empty space
named as specified.`,
	New: New,
}

func init() {
	apicmd.Cmd.Add(NewSpec)
}

type Command struct {
	*apicmd.Command
	createFlags apicmd.SpaceCreateFlags
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*apicmd.Command)}
	c.createFlags.SetFlags(f)
	return c, nil
}

func (c *Command) Run(args []string) error {
	if len(args) > 1 {
		return errors.New("too many arguments")
	}
	if len(args) != 1 {
		return errors.New("must specify a space name")
	}
	defer c.Cleanup()
	if err := c.Init(&c.createFlags); err != nil {
		return err
	}

	name := args[0]
	conn := c.Connection()
	sp, err := c.createFlags.Create(c.Context(), conn, name)
	if err != nil {
		return fmt.Errorf("couldn't create new space %s: %w", name, err)
	}
	fmt.Printf("%s: space created\n", sp.Name)
	return nil
}
