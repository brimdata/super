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
The use command prints or sets the working pool and branch.  Setting these
values allows commands like load, rebase, merge, etc. to function without
having to specify the working branch.  The branch specifier may also be
a commit ID, in which case you enter a headless state and commands
like load that require a branch will report an error.

The use command is like "git checkout" but there is no local copy of
the database.  Rather, the local HEAD state influences commands as
they access the database.

With no argument, use prints the working pool and branch as well as the
location of the current database.

With an argument of the form "pool", use sets the working pool as indicated
and the working branch to "main".

With an argument of the form "pool@branch", use sets the working pool and
branch as indicated.

With an argument of the form "@branch", use sets only the working branch.
The working pool must already be set.

The pool must be the name or ID of an existing pool.  The branch must be
the name of an existing branch or a commit ID.

Any command that relies upon HEAD can also be run with the -use option
to refer to a different HEAD without executing an explicit "use" command.
While the use of HEAD is convenient for interactive CLI sessions,
automation and orchestration tools are better off hard-wiring the
HEAD references in each database command via -use.

The use command merely checks that the branch exists and updates the
file ~/.zed_head.  This file simply contains a pointer to the HEAD branch
and thus provides the default for the -use option.  This way, multiple working
directories can contain different HEAD pointers (along with your local files)
and you can easily switch between windows without having to continually
re-specify a new HEAD.  Unlike Git, all the committed pool data remains
in the database and is not copied to this local directory.
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
