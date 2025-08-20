package db

import (
	"errors"
	"flag"
	"os"

	"github.com/brimdata/super/cli/lakeflags"
	"github.com/brimdata/super/cli/outputflags"
	"github.com/brimdata/super/cli/queryflags"
	"github.com/brimdata/super/cli/runtimeflags"
	"github.com/brimdata/super/cmd/super/root"
	"github.com/brimdata/super/pkg/charm"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/supio"
	"github.com/brimdata/super/zbuf"
)

var Spec = &charm.Spec{
	Name:  "db",
	Usage: "db <sub-command> [options] [arguments...]",
	Short: "run SuperDB data lake commands",
	Long: `
XXX db is a command-line tool for creating, configuring, ingesting into,
querying, and orchestrating Zed data lakes.`,
	New:          New,
	InternalLeaf: true,
}

func init() {
	root.Super.Add(Spec)
}

type Command struct {
	*root.Command
	LakeFlags    lakeflags.Flags
	outputFlags  outputflags.Flags
	queryFlags   queryflags.Flags
	runtimeFlags runtimeflags.Flags
	query        string
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*root.Command)}
	c.LakeFlags.SetFlags(f)
	return c, nil
}

func (c *Command) SetLeafFlags(f *flag.FlagSet) {
	c.outputFlags.SetFlags(f)
	c.queryFlags.SetFlags(f)
	c.runtimeFlags.SetFlags(f)
	f.StringVar(&c.query, "c", "", "query to execute")
}

func (c *Command) Run(args []string) error {
	ctx, cleanup, err := c.Init(&c.outputFlags, &c.runtimeFlags)
	if err != nil {
		return err
	}
	defer cleanup()
	if len(args) == 0 && len(c.queryFlags.Includes) == 0 && c.query == "" {
		return charm.NeedHelp
	}
	if len(args) > 0 {
		return errors.New("super db command takes no arguments")
	}
	lake, err := c.LakeFlags.Open(ctx)
	if err != nil {
		return err
	}
	w, err := c.outputFlags.Open(ctx, storage.NewLocalEngine())
	if err != nil {
		return err
	}
	query, err := lake.Query(ctx, c.query, c.queryFlags.Includes...)
	if err != nil {
		w.Close()
		return err
	}
	defer query.Pull(true)
	out := map[string]sio.WriteCloser{
		"main":  w,
		"debug": supio.NewWriter(sio.NopCloser(os.Stderr), supio.WriterOpts{}),
	}
	err = zbuf.CopyMux(out, query)
	if closeErr := w.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		c.queryFlags.PrintStats(query.Progress())
	}
	return err
}
