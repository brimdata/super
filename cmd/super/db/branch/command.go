package branch

import (
	"context"
	"errors"
	"flag"
	"fmt"

	"github.com/brimdata/super/cli/outputflags"
	"github.com/brimdata/super/cli/poolflags"
	"github.com/brimdata/super/cmd/super/db"
	"github.com/brimdata/super/db/api"
	"github.com/brimdata/super/dbid"
	"github.com/brimdata/super/pkg/charm"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/zbuf"
)

var spec = &charm.Spec{
	Name:  "branch",
	Usage: "branch new-branch [base]",
	Short: "create a new branch",
	Long: `
The db branch command creates a new branch with the indicated name.
If specified, base is either an existing branch name or a commit ID
and provides the new branch's base.  If not specified, then HEAD is assumed.

The branch command does not check out the new branch.

If the -d option is specified, then the branch is deleted.  No data is
deleted by this operation and the deleted branch can be easily recreated by
running the branch command again with the commit ID desired.

If no branch is currently checked out, then "-use pool@base" can be
supplied to specify the desired pool for the new branch.
`,
	New: New,
}

type Command struct {
	*db.Command
	delete      bool
	outputFlags outputflags.Flags
	poolFlags   poolflags.Flags
}

func init() {
	db.Spec.Add(spec)
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*db.Command)}
	f.BoolVar(&c.delete, "d", false, "delete the branch instead of creating it")
	c.outputFlags.DefaultFormat = "db"
	c.outputFlags.SetFlags(f)
	c.poolFlags.SetFlags(f)
	return c, nil
}

func (c *Command) Run(args []string) error {
	ctx, cleanup, err := c.Init(&c.outputFlags)
	if err != nil {
		return err
	}
	if len(args) > 1 {
		return errors.New("too many arguments")
	}
	defer cleanup()
	db, err := c.DBFlags.Open(ctx)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		return c.list(ctx, db)
	}
	branchName := args[0]
	head, err := c.poolFlags.HEAD()
	if err != nil {
		return err
	}
	poolName := head.Pool
	if poolName == "" {
		return errors.New("a pool name must be included: pool@branch")
	}
	poolID, err := dbid.ParseID(poolName)
	if err != nil {
		poolID, err = db.PoolID(ctx, poolName)
		if err != nil {
			return err
		}
	}
	parentCommit, err := dbid.ParseID(head.Branch)
	if err != nil {
		parentCommit, err = db.CommitObject(ctx, poolID, head.Branch)
		if err != nil {
			return err
		}
	}
	if c.delete {
		if err := db.RemoveBranch(ctx, poolID, branchName); err != nil {
			return err
		}
		if !c.DBFlags.Quiet {
			fmt.Printf("branch deleted: %s\n", branchName)
		}
		return nil
	}
	if err := db.CreateBranch(ctx, poolID, branchName, parentCommit); err != nil {
		return err
	}
	if !c.DBFlags.Quiet {
		fmt.Printf("%q: branch created\n", branchName)
	}
	return nil
}

func (c *Command) list(ctx context.Context, db api.Interface) error {
	head, err := c.poolFlags.HEAD()
	if err != nil {
		return err
	}
	poolName := head.Pool
	if poolName == "" {
		return errors.New("must be on a checked out branch to list the branches in the same pool")
	}
	query := fmt.Sprintf("from '%s':branches", poolName)
	if c.outputFlags.Format == "db" {
		c.outputFlags.WriterOpts.DB.Head = head.Branch
	}
	w, err := c.outputFlags.Open(ctx, storage.NewLocalEngine())
	if err != nil {
		return err
	}
	q, err := db.Query(ctx, query)
	if err != nil {
		w.Close()
		return err
	}
	defer q.Pull(true)
	err = zbuf.CopyPuller(w, q)
	if closeErr := w.Close(); err == nil {
		err = closeErr
	}
	return err
}
