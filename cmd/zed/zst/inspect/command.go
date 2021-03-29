package inspect

import (
	"context"
	"errors"
	"flag"

	"github.com/brimdata/zq/cli/outputflags"
	zedzst "github.com/brimdata/zq/cmd/zed/zst"
	zstcmd "github.com/brimdata/zq/cmd/zed/zst"
	"github.com/brimdata/zq/pkg/charm"
	"github.com/brimdata/zq/zbuf"
	"github.com/brimdata/zq/zng/resolver"
	"github.com/brimdata/zq/zst"
)

var Inspect = &charm.Spec{
	Name:  "inspect",
	Usage: "inspect [flags] file",
	Short: "look at info in a zst file",
	Long: `
The inspect command extracts information from a zst file.
This is mostly useful for test and debug, though there may be interesting
uses as the zst format becomes richer with pruning information and other internal
aggregations about the columns and so forth.

The -R option (on by default) sends the reassembly records to the output while
the -trailer option (off by defaulut) indicates that the trailer should be included.
`,
	New: newCommand,
}

func init() {
	zedzst.Cmd.Add(Inspect)
}

type Command struct {
	*zedzst.Command
	outputFlags outputflags.Flags
	trailer     bool
	reassembly  bool
}

func newCommand(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*zstcmd.Command)}
	f.BoolVar(&c.trailer, "trailer", false, "include the zst trailer in the output")
	f.BoolVar(&c.reassembly, "R", true, "include the zst reassembly section in the output")
	c.outputFlags.SetFlags(f)
	return c, nil
}

func (c *Command) Run(args []string) error {
	defer c.Cleanup()
	if err := c.Init(&c.outputFlags); err != nil {
		return err
	}
	if len(args) != 1 {
		return errors.New("zst inspect: must be run with a single file argument")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	path := args[0]
	reader, err := zst.NewReaderFromPath(ctx, resolver.NewContext(), path)
	if err != nil {
		return err
	}
	defer reader.Close()
	writer, err := c.outputFlags.Open(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if writer != nil {
			writer.Close()
		}
	}()
	if c.reassembly {
		r := reader.NewReassemblyReader()
		if err := zbuf.Copy(writer, r); err != nil {
			return err
		}
	}
	if c.trailer {
		r := reader.NewTrailerReader()
		if err := zbuf.Copy(writer, r); err != nil {
			return err
		}
	}
	err = writer.Close()
	writer = nil
	return err
}
