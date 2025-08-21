package compact

import (
	"flag"
	"fmt"

	"github.com/brimdata/super/cli/commitflags"
	"github.com/brimdata/super/cli/poolflags"
	"github.com/brimdata/super/cmd/super/db"
	"github.com/brimdata/super/dbid"
	"github.com/brimdata/super/pkg/charm"
)

var spec = &charm.Spec{
	Name:  "compact",
	Usage: "compact id id [id ...]",
	Short: "compact data objects on a pool branch",
	Long: `
The compact command takes a list of data object IDs, writes the values
in those objects to a sequence of new, non-overlapping objects, and creates
a commit on HEAD replacing the old objects with the new ones.`,
	New: New,
}

type Command struct {
	*db.Command
	commitFlags  commitflags.Flags
	poolFlags    poolflags.Flags
	writeVectors bool
}

func init() {
	db.Spec.Add(spec)
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*db.Command)}
	c.commitFlags.SetFlags(f)
	c.poolFlags.SetFlags(f)
	f.BoolVar(&c.writeVectors, "vectors", false, "write vectors for compacted objects")
	return c, nil
}

func (c *Command) Run(args []string) error {
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
	poolID, err := db.PoolID(ctx, head.Pool)
	if err != nil {
		return err
	}
	commit, err := db.Compact(ctx, poolID, head.Branch, ids, c.writeVectors, c.commitFlags.CommitMessage())
	if err == nil && !c.DBFlags.Quiet {
		fmt.Printf("%s compaction committed\n", commit)
	}
	return err
}
