package vector

import (
	"flag"
	"fmt"

	"github.com/brimdata/super/cli/commitflags"
	"github.com/brimdata/super/cli/poolflags"
	"github.com/brimdata/super/dbid"
	"github.com/brimdata/super/pkg/charm"
)

var del = &charm.Spec{
	Name:  "delete",
	Usage: "delete [options] id [id, ]",
	Short: "deleted vectors from one or more data objects",
	Long: `
The vector delete command deletes vectors from of one or more data objects specified
by the indicated object IDs.  The references to the vectors is simply deleted
in the commit history.  The vacate command may be used to delete the actual data.
`,
	New: newDelete,
}

type deleteCommand struct {
	*Command
	commitFlags commitflags.Flags
	poolFlags   poolflags.Flags
}

func newDelete(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &deleteCommand{Command: parent.(*Command)}
	c.commitFlags.SetFlags(f)
	c.poolFlags.SetFlags(f)
	return c, nil
}

func (c *deleteCommand) Run(args []string) error {
	ctx, cleanup, err := c.Init()
	if err != nil {
		return err
	}
	defer cleanup()
	ids, err := dbid.ParseIDs(args)
	if err != nil {
		return err
	}
	db, err := c.DBFlags.Open(ctx)
	if err != nil {
		return err
	}
	head, err := c.poolFlags.HEAD()
	if err != nil {
		return err
	}
	commit, err := db.DeleteVectors(ctx, head.Pool, head.Branch, ids, c.commitFlags.CommitMessage())
	if err == nil && !c.DBFlags.Quiet {
		fmt.Printf("%s vectors deleted\n", commit)
	}
	return err
}
