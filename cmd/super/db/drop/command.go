package drop

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/brimdata/super/cmd/super/db"
	"github.com/brimdata/super/pkg/charm"
)

var spec = &charm.Spec{
	Name:  "drop",
	Usage: "drop pool",
	Short: "delete a data pool from a database",
	Long: `
The drop command removes the named pool from the database.

DANGER ZONE.
When deleting an entire pool, the drop command prompts for confirmation.
Once the pool is deleted, its data is gone so use this command carefully.
`,
	New: New,
}

func init() {
	db.Spec.Add(spec)
}

type Command struct {
	*db.Command
	force bool
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*db.Command)}
	f.BoolVar(&c.force, "f", false, "do not prompt for confirmation")
	return c, nil
}

func (c *Command) Run(args []string) error {
	ctx, cleanup, err := c.Init()
	if err != nil {
		return err
	}
	defer cleanup()
	if len(args) != 1 {
		return errors.New("a single pool name must be specified")
	}
	db, err := c.DBFlags.Open(ctx)
	if err != nil {
		return err
	}
	poolName := args[0]
	poolID, err := db.PoolID(ctx, poolName)
	if err != nil {
		return err
	}
	if err := c.confirm(poolName); err != nil {
		return err
	}
	if err := db.RemovePool(ctx, poolID); err != nil {
		return err
	}
	if !c.DBFlags.Quiet {
		fmt.Printf("pool deleted: %s\n", poolName)
	}
	return nil
}

func (c *Command) confirm(name string) error {
	if c.force {
		return nil
	}
	fmt.Printf("Are you sure you want to delete pool %q? There is no going back... [y|n]\n", name)
	var input string
	if _, err := fmt.Scanln(&input); err != nil {
		return err
	}
	input = strings.ToLower(input)
	if input == "y" || input == "yes" {
		return nil
	}
	return errors.New("operation canceled")
}
