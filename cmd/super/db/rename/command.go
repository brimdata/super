package rename

import (
	"errors"
	"flag"
	"fmt"

	"github.com/brimdata/super/cmd/super/db"
	"github.com/brimdata/super/pkg/charm"
)

var spec = &charm.Spec{
	Name:  "rename",
	Usage: "rename old-name new-name",
	Short: "rename a data pool",
	Long: `
The rename command changes the name of the pool given by the -p option to the
new name provided.
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
	c := &Command{Command: parent.(*db.Command)}
	return c, nil
}

func (c *Command) Run(args []string) error {
	ctx, cleanup, err := c.Init()
	if err != nil {
		return err
	}
	defer cleanup()
	if len(args) != 2 {
		return errors.New("two pool names must be provided")
	}
	oldName := args[0]
	newName := args[1]
	db, err := c.DBFlags.Open(ctx)
	if err != nil {
		return err
	}
	poolID, err := db.PoolID(ctx, oldName)
	if err != nil {
		return err
	}
	if err := db.RenamePool(ctx, poolID, newName); err != nil {
		return err
	}
	if !c.DBFlags.Quiet {
		fmt.Printf("pool %s renamed from %s to %s\n", poolID, oldName, newName)
	}
	return nil
}
