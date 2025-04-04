package log

import (
	"errors"
	"flag"

	"github.com/brimdata/super/cli/outputflags"
	"github.com/brimdata/super/cli/poolflags"
	"github.com/brimdata/super/cmd/super/db"
	"github.com/brimdata/super/pkg/charm"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/zbuf"
)

var spec = &charm.Spec{
	Name:  "log",
	Usage: "log [options]",
	Short: "display the commit log history starting at any commit",
	Long: `
The log command outputs a commit history of any branch or unnamed commit object
from a data pool in the format desired.
By default, the output is in the human-readable "lake" format
but BSUP can be used to easily be pipe a log to super or other tooling for analysis.
`,
	New: New,
}

func init() {
	db.Spec.Add(spec)
}

type Command struct {
	*db.Command
	outputFlags outputflags.Flags
	poolFlags   poolflags.Flags
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*db.Command)}
	c.outputFlags.DefaultFormat = "lake"
	c.outputFlags.SetFlags(f)
	c.poolFlags.SetFlags(f)
	return c, nil
}

func (c *Command) Run(args []string) error {
	ctx, cleanup, err := c.Init(&c.outputFlags)
	if err != nil {
		return err
	}
	defer cleanup()
	if len(args) != 0 {
		return errors.New("no arguments allowed")
	}
	lake, err := c.LakeFlags.Open(ctx)
	if err != nil {
		return err
	}
	head, err := c.poolFlags.HEAD()
	if err != nil {
		return err
	}
	query, err := head.FromSpec("log")
	if err != nil {
		return err
	}
	if c.outputFlags.Format == "lake" {
		c.outputFlags.WriterOpts.Lake.Head = head.Branch
	}
	w, err := c.outputFlags.Open(ctx, storage.NewLocalEngine())
	if err != nil {
		return err
	}
	defer w.Close()
	q, err := lake.Query(ctx, query)
	if err != nil {
		return err
	}
	defer q.Pull(true)
	err = zbuf.CopyPuller(w, q)
	if closeErr := w.Close(); err == nil {
		err = closeErr
	}
	return err
}
