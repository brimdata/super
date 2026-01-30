package use

import (
	"errors"
	"flag"
	"fmt"

	"github.com/brimdata/super/cli/poolflags"
	"github.com/brimdata/super/cmd/super/db"
	"github.com/brimdata/super/dbid"
	"github.com/brimdata/super/pkg/charm"
)

//XXX should use be called connect?

var spec = &charm.Spec{
	Name:  "use",
	Usage: "use [pool][@branch]",
	Short: "use a branch or print current branch and database",
	Long: `
See https://superdb.org/command/db.html#super-db-use
`,
	New: New,
}

func init() {
	db.Spec.Add(spec)
}

type Command struct {
	*db.Command
	poolFlags poolflags.Flags
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*db.Command)}
	c.poolFlags.SetFlags(f)
	return c, nil
}

func (c *Command) Run(args []string) error {
	ctx, cleanup, err := c.Init()
	if err != nil {
		return err
	}
	defer cleanup()
	if len(args) > 1 {
		return errors.New("too many arguments")
	}
	if len(args) == 0 {
		head, err := c.poolFlags.HEAD()
		if err != nil {
			return errors.New("default pool and branch unset")
		}
		fmt.Printf("HEAD at %s\n", head)
		if u, err := c.DBFlags.ClientURI(); err == nil {
			fmt.Printf("Database at %s\n", u)
		}
		return nil
	}
	commitish, err := dbid.ParseCommitish(args[0])
	if err != nil {
		return err
	}
	if commitish.Pool == "" {
		head, err := c.poolFlags.HEAD()
		if err != nil {
			return errors.New("default pool unset")
		}
		commitish.Pool = head.Pool
	}
	if commitish.Branch == "" {
		commitish.Branch = "main"
	}
	db, err := c.DBFlags.Open(ctx)
	if err != nil {
		return err
	}
	poolID, err := db.PoolID(ctx, commitish.Pool)
	if err != nil {
		return err
	}
	if _, err = db.CommitObject(ctx, poolID, commitish.Branch); err != nil {
		return err
	}
	if err := poolflags.WriteHead(commitish.Pool, commitish.Branch); err != nil {
		return err
	}
	if !c.DBFlags.Quiet {
		fmt.Printf("Switched to branch %q on pool %q\n", commitish.Branch, commitish.Pool)
	}
	return nil
}
