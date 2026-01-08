package db

import (
	"errors"
	"flag"
	"os"

	"github.com/brimdata/super/cli/dbflags"
	"github.com/brimdata/super/cli/outputflags"
	"github.com/brimdata/super/cli/queryflags"
	"github.com/brimdata/super/cli/runtimeflags"
	"github.com/brimdata/super/cmd/super/root"
	"github.com/brimdata/super/pkg/charm"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/supio"
)

var Spec = &charm.Spec{
	Name:  "db",
	Usage: "db <sub-command> [options] [arguments...]",
	Short: "run database commands",
	Long: `
db is a command-line tool for creating, configuring, ingesting into,
querying, and orchestrating databases.`,
	New:          New,
	InternalLeaf: true,
}

func init() {
	root.Super.Add(Spec)
}

type Command struct {
	*root.Command
	DBFlags      dbflags.Flags
	outputFlags  outputflags.Flags
	queryFlags   queryflags.Flags
	runtimeFlags runtimeflags.Flags
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*root.Command)}
	c.DBFlags.SetFlags(f)
	return c, nil
}

func (c *Command) SetLeafFlags(f *flag.FlagSet) {
	c.outputFlags.SetFlags(f)
	c.queryFlags.SetFlags(f)
	c.runtimeFlags.SetFlags(f)
}

func (c *Command) Run(args []string) error {
	ctx, cleanup, err := c.Init(&c.outputFlags, &c.runtimeFlags)
	if err != nil {
		return err
	}
	defer cleanup()
	if len(args) == 0 && len(c.queryFlags.Query) == 0 {
		return charm.NeedHelp
	}
	if len(args) > 0 {
		return errors.New("super db command takes no arguments")
	}
	db, err := c.DBFlags.Open(ctx)
	if err != nil {
		return err
	}
	w, err := c.outputFlags.Open(ctx, storage.NewLocalEngine())
	if err != nil {
		return err
	}
	query, err := db.Query(ctx, c.queryFlags.Query)
	if err != nil {
		w.Close()
		return err
	}
	defer query.Pull(true)
	out := map[string]sio.WriteCloser{
		"main":  w,
		"debug": supio.NewWriter(sio.NopCloser(os.Stderr), supio.WriterOpts{}),
	}
	err = sbuf.CopyMux(out, query)
	if closeErr := w.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		c.queryFlags.PrintStats(query.Progress())
	}
	return err
}
