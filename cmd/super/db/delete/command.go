package del

import (
	"context"
	"errors"
	"flag"
	"fmt"

	"github.com/brimdata/super/cli/commitflags"
	"github.com/brimdata/super/cli/dbflags"
	"github.com/brimdata/super/cli/poolflags"
	"github.com/brimdata/super/cmd/super/db"
	"github.com/brimdata/super/db/api"
	"github.com/brimdata/super/dbid"
	"github.com/brimdata/super/pkg/charm"
	"github.com/segmentio/ksuid"
)

var spec = &charm.Spec{
	Name:  "delete",
	Usage: "delete id [id ...]",
	Short: "delete data objects from a pool branch",
	Long: `
The delete command takes a list of data object IDs and
deletes references to those object from HEAD by commiting a new
delete operation to HEAD.
Once the delete operation completes, the deleted data is no longer seen
when read data from the pool.

If the -where flag is specified, delete will remove all values for which the
provided filter expression is true. The value provided to where must be a
single filter expression, e.g.:

zed delete -where 'ts > 2022-10-05T17:20:00Z and ts < 2022-10-05T17:21:00Z'

No data is actually removed from the database.  Instead, a delete
operation is an action in the pool's commit journal.  Any delete
can be "undone" by adding the commits back to the log using
"zed revert".
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
	where       string
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*db.Command)}
	c.commitFlags.SetFlags(f)
	c.poolFlags.SetFlags(f)
	f.StringVar(&c.where, "where", "", "delete by pool key predicate")
	return c, nil
}

func (c *Command) Run(args []string) error {
	ctx, cleanup, err := c.Init()
	if err != nil {
		return err
	}
	defer cleanup()
	db, err := c.DBFlags.Open(ctx)
	if err != nil {
		return err
	}
	head, err := c.poolFlags.HEAD()
	if err != nil {
		return err
	}
	poolName := head.Pool
	if poolName == "" {
		return dbflags.ErrNoHEAD
	}
	poolID, err := db.PoolID(ctx, poolName)
	if err != nil {
		return err
	}
	var commit ksuid.KSUID
	if c.where != "" {
		if len(args) > 0 {
			return errors.New("too many arguments")
		}
		commit, err = c.deleteWhere(ctx, db, poolID, head.Branch)
	} else {
		commit, err = c.deleteByIDs(ctx, db, poolID, head.Branch, args)
	}
	if err != nil {
		return err
	}
	if !c.DBFlags.Quiet {
		fmt.Printf("%s delete committed\n", commit)
	}
	return nil
}

func (c *Command) deleteByIDs(ctx context.Context, db api.Interface, poolID ksuid.KSUID, branchName string, args []string) (ksuid.KSUID, error) {
	ids, err := dbid.ParseIDs(args)
	if err != nil {
		return ksuid.Nil, err
	}
	if len(ids) == 0 {
		return ksuid.Nil, errors.New("no data object IDs specified")
	}
	return db.Delete(ctx, poolID, branchName, ids, c.commitFlags.CommitMessage())
}

func (c *Command) deleteWhere(ctx context.Context, db api.Interface, poolID ksuid.KSUID, branchName string) (ksuid.KSUID, error) {
	return db.DeleteWhere(ctx, poolID, branchName, c.where, c.commitFlags.CommitMessage())
}
