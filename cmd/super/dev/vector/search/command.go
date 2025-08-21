package search

import (
	"errors"
	"flag"

	"github.com/brimdata/super"
	"github.com/brimdata/super/cli/dbflags"
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
	Short: "run a CSUP optimized search on a database",
	New:   newCommand,
}

func init() {
	vector.Spec.Add(spec)
}

type Command struct {
	*vector.Command
	dbFlags     dbflags.Flags
	outputFlags outputflags.Flags
	poolFlags   poolflags.Flags
}

func newCommand(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*vector.Command)}
	c.dbFlags.SetFlags(f)
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
	if len(args) != 1 {
		return errors.New("usage: filter expression")
	}
	db, err := c.dbFlags.Open(ctx)
	if err != nil {
		return err
	}
	root := db.Root()
	if root == nil {
		return errors.New("remote databases not supported")
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
