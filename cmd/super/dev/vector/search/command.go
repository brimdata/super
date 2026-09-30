package search

import (
	"errors"
	"flag"

	"github.com/superdb/super"
	"github.com/superdb/super/cli/dbflags"
	"github.com/superdb/super/cli/outputflags"
	"github.com/superdb/super/cli/poolflags"
	"github.com/superdb/super/cmd/super/dev/vector"
	"github.com/superdb/super/compiler"
	"github.com/superdb/super/pkg/charm"
	"github.com/superdb/super/pkg/storage"
	"github.com/superdb/super/runtime"
	"github.com/superdb/super/runtime/exec"
	"github.com/superdb/super/sbuf"
	"github.com/superdb/super/vector/vio"
)

var spec = &charm.Spec{
	Name:  "search",
	Usage: "search [flags] filter_expr",
	Short: "run a BSUP optimized search on a database",
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
	sctx := super.NewContext()
	rctx := runtime.NewContext(ctx, sctx)
	puller, err := compiler.VectorFilterCompile(rctx, text, exec.NewEnvironment(nil, root), head)
	if err != nil {
		return err
	}
	writer, err := c.outputFlags.Open(ctx, storage.NewLocalEngine())
	if err != nil {
		return err
	}
	if err := vio.Copy(writer, sbuf.NewDematerializer(sctx, puller)); err != nil {
		writer.Close()
		return err
	}
	return writer.Close()
}
