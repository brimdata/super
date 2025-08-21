package revert

import (
	"errors"
	"flag"
	"fmt"

	"github.com/brimdata/super/cli/commitflags"
	"github.com/brimdata/super/cli/dbflags"
	"github.com/brimdata/super/cli/poolflags"
	"github.com/brimdata/super/cmd/super/db"
	"github.com/brimdata/super/dbid"
	"github.com/brimdata/super/pkg/charm"
)

var spec = &charm.Spec{
	Name:  "revert",
	Usage: "revert commit",
	Short: "revert reverses an old commit",
	Long: `
The revert command reverses the actions in a commit by applying the inverse
steps in a new commit to the tip of the indicated branch.  Any data loaded
in a reverted commit remains in the database but no longer appears in the branch.
The new commit may recursively be reverted by an additional revert operation.
`,
	New: New,
}

func init() {
	db.Spec.Add(spec)
}

type Command struct {
	*db.Command
	commitFlags commitflags.Flags
	poolFlags   poolflags.Flags
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*db.Command)}
	c.commitFlags.SetFlags(f)
	c.poolFlags.SetFlags(f)
	return c, nil
}

func (c *Command) Run(args []string) error {
	ctx, cleanup, err := c.Init()
	if err != nil {
		return err
	}
	defer cleanup()
	if len(args) != 1 {
		return errors.New("commit ID must be specified")
	}
	db, err := c.DBFlags.Open(ctx)
	if err != nil {
		return err
	}
	head, err := c.poolFlags.HEAD()
	if err != nil {
		return err
	}
	if head.Pool == "" {
		return dbflags.ErrNoHEAD
	}
	poolID, err := db.PoolID(ctx, head.Pool)
	if err != nil {
		return err
	}
	if _, err := dbid.ParseID(head.Branch); err == nil {
		return errors.New("branch must be named")
	}
	commitID, err := dbid.ParseID(args[0])
	if err != nil {
		return err
	}
	revertID, err := db.Revert(ctx, poolID, head.Branch, commitID, c.commitFlags.CommitMessage())
	if err != nil {
		return err
	}
	if !c.DBFlags.Quiet {
		fmt.Printf("%q: %s reverted in %s\n", head.Branch, commitID, revertID)
	}
	return nil
}
