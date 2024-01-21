package copy

import (
	"errors"
	"flag"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/cli/outputflags"
	devvcache "github.com/brimdata/zed/cmd/zed/dev/vcache"
	"github.com/brimdata/zed/cmd/zed/root"
	"github.com/brimdata/zed/pkg/charm"
	"github.com/brimdata/zed/pkg/storage"
	"github.com/brimdata/zed/runtime/vam"
	"github.com/brimdata/zed/runtime/vcache"
	"github.com/brimdata/zed/zbuf"
)

var Copy = &charm.Spec{
	Name:  "copy",
	Usage: "copy [flags] path",
	Short: "read a VNG file and copy to the output through the vector cache",
	Long: `
The copy command reads VNG vectors from
a VNG storage objects (local files or s3 objects) and outputs
the reconstructed ZNG row data by exercising the vector cache.

This command is most useful for testing the VNG vector cache.
`,
	New: newCommand,
}

func init() {
	devvcache.Cmd.Add(Copy)
}

type Command struct {
	*root.Command
	outputFlags outputflags.Flags
}

func newCommand(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*root.Command)}
	c.outputFlags.SetFlags(f)
	return c, nil
}

func (c *Command) Run(args []string) error {
	ctx, cleanup, err := c.Init(&c.outputFlags)
	if err != nil {
		return err
	}
	defer cleanup()
	if len(args) != 1 {
		return errors.New("VNG read: must be run with a single path argument")
	}
	uri, err := storage.ParseURI(args[0])
	if err != nil {
		return err
	}
	local := storage.NewLocalEngine()
	object, err := vcache.NewObject(ctx, local, uri)
	if err != nil {
		return err
	}
	defer object.Close()
	writer, err := c.outputFlags.Open(ctx, local)
	if err != nil {
		return err
	}
	puller := vam.NewProjection(zed.NewContext(), object, nil)
	if err := zbuf.CopyPuller(writer, puller); err != nil {
		writer.Close()
		return err
	}
	return writer.Close()
}
