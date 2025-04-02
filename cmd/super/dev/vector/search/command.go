package search

import (
	"errors"
	"flag"

	"github.com/brimdata/super"
	"github.com/brimdata/super/cli/lakeflags"
	"github.com/brimdata/super/cli/outputflags"
	"github.com/brimdata/super/cli/poolflags"
	"github.com/brimdata/super/cmd/super/dev/vector"
	"github.com/brimdata/super/compiler"
	"github.com/brimdata/super/pkg/charm"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/exec"
	"github.com/brimdata/super/zbuf"
)

var spec = &charm.Spec{
	Name:  "search",
	Usage: "search [flags] filter_expr",
	Short: "run a CSUP optimized search on a lake",
	New:   newCommand,
}

func init() {
	vector.Spec.Add(spec)
}

type Command struct {
	*vector.Command
	outputFlags outputflags.Flags
	poolFlags   poolflags.Flags
	lakeFlags   lakeflags.Flags
}

func newCommand(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*vector.Command)}
	c.outputFlags.SetFlags(f)
	c.poolFlags.SetFlags(f)
	c.lakeFlags.SetFlags(f)
	return c, nil
}

func (c *Command) Run(args []string) error {
	ctx, cleanup, err := c.Init(&c.outputFlags)
	if err != nil {
		return err
	}
	defer cleanup()
	if len(args) != 1 {
		return errors.New("usage: filter expression")
	}
	lk, err := c.lakeFlags.Open(ctx)
	if err != nil {
		return err
	}
	root := lk.Root()
	if root == nil {
		return errors.New("remote lakes not supported")
	}
	head, err := c.poolFlags.HEAD()
	if err != nil {
		return err
	}
	text := args[0]
	rctx := runtime.NewContext(ctx, super.NewContext())
	puller, err := compiler.VectorFilterCompile(rctx, text, exec.NewEnvironment(nil, root), head)
	if err != nil {
		return err
	}
	writer, err := c.outputFlags.Open(ctx, storage.NewLocalEngine())
	if err != nil {
		return err
	}
	if err := zbuf.CopyPuller(writer, puller); err != nil {
		writer.Close()
		return err
	}
	return writer.Close()
}
