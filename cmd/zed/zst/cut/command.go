package inspect

import (
	"context"
	"errors"
	"flag"
	"strings"

	"github.com/brimsec/zq/cli/outputflags"
	zstcmd "github.com/brimsec/zq/cmd/zed/zst"
	"github.com/brimsec/zq/pkg/charm"
	"github.com/brimsec/zq/zbuf"
	"github.com/brimsec/zq/zng/resolver"
	"github.com/brimsec/zq/zst"
)

var Cut = &charm.Spec{
	Name:  "cut",
	Usage: "cut [flags] -k field-expr path",
	Short: "cut a column from a zst file",
	Long: `
The cut command cuts a single column from a zst file and writes the column
to the output in the format of choice.

This command is most useful for test, debug, and demo, as more efficient
and complete "cuts" on zst files will eventually be available from zq
in the future.  For example, zq cut will optmize the query

	count() by _path

to cut the path field and run analytics directly on the result without having
to scan all of the zng row data.
`,
	New: newCommand,
}

func init() {
	zstcmd.Cmd.Add(Cut)
}

type Command struct {
	*zstcmd.Command
	outputFlags outputflags.Flags
	fieldExpr   string
}

func newCommand(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*zstcmd.Command)}
	f.StringVar(&c.fieldExpr, "k", "", "dotted field expression of field to cut")
	c.outputFlags.SetFlags(f)
	return c, nil
}

func (c *Command) Run(args []string) error {
	defer c.Cleanup()
	if err := c.Init(&c.outputFlags); err != nil {
		return err
	}
	if len(args) != 1 {
		return errors.New("zst cut: must be run with a single input file")
	}
	if c.fieldExpr == "" {
		return errors.New("zst cut: must specify field to cut with -k")
	}
	fields := strings.Split(c.fieldExpr, ".")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	path := args[0]
	cutter, err := zst.NewCutterFromPath(ctx, resolver.NewContext(), path, fields)
	if err != nil {
		return err
	}
	defer cutter.Close()
	writer, err := c.outputFlags.Open(ctx)
	if err != nil {
		return err
	}
	if err := zbuf.Copy(writer, cutter); err != nil {
		writer.Close()
		return err
	}
	return writer.Close()
}
