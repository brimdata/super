package vacate

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/brimdata/zed/cli/poolflags"
	"github.com/brimdata/zed/cmd/zed/root"
	"github.com/brimdata/zed/lake/api"
	"github.com/brimdata/zed/lakeparse"
	"github.com/brimdata/zed/pkg/charm"
	"github.com/brimdata/zed/pkg/nano"
	"github.com/brimdata/zed/pkg/plural"
)

var Cmd = &charm.Spec{
	Name:  "vacate",
	Usage: "vacate [options] [timestamp]",
	Short: "compact a pool's commit history by removing old commit objects",
	Long: `
See https://superdb.org/command/db.html#super-db-vacate
`,
	New: New,
}

type Command struct {
	*root.Command
	poolFlags poolflags.Flags
	dryrun    bool
	force     bool
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*root.Command)}
	c.poolFlags.SetFlags(f)
	f.BoolVar(&c.dryrun, "dryrun", false, "view the number of commits to be deleted")
	f.BoolVar(&c.force, "f", false, "do not prompt for confirmation")
	return c, nil
}

func (c *Command) Run(args []string) error {
	ctx, cleanup, err := c.Init()
	if err != nil {
		return err
	}
	defer cleanup()
	db, err := c.LakeFlags.Open(ctx)
	if err != nil {
		return err
	}
	at, err := c.poolFlags.HEAD()
	if err != nil {
		return err
	}
	var ts nano.Ts
	if len(args) > 0 {
		ts, err = nano.ParseRFC3339Nano([]byte(args[0]))
	} else {
		ts, err = c.getTsFromCommitish(ctx, db, at)
	}
	if err != nil {
		return err
	}
	verb := "would vacate"
	if !c.dryrun {
		verb = "vacated"
		if err := c.confirm(ctx, ts); err != nil {
			return err
		}
	}
	cids, err := db.Vacate(ctx, at.Pool, ts, c.dryrun)
	if err != nil {
		return err
	}
	if !c.LakeFlags.Quiet {
		fmt.Printf("%s %d commit%s\n", verb, len(cids), plural.Slice(cids, "s"))
	}
	return nil
}

func (c *Command) getTsFromCommitish(ctx context.Context, db api.Interface, at *lakeparse.Commitish) (nano.Ts, error) {
	commit, err := api.GetCommit(ctx, db, at.Pool, at.Branch)
	if err != nil {
		return 0, err
	}
	return commit.Date, nil
}

func (c *Command) confirm(ctx context.Context, ts nano.Ts) error {
	if c.force {
		return nil
	}
	fmt.Printf("Are you sure you want to vacate history order than %s? There is no going back... [y|n]\n", ts)
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
