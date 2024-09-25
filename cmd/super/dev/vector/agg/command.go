package agg

import (
	"errors"
	"flag"

	zed "github.com/brimdata/super"
	"github.com/brimdata/super/cli/outputflags"
	"github.com/brimdata/super/cmd/super/dev/vector"
	"github.com/brimdata/super/cmd/super/root"
	"github.com/brimdata/super/pkg/charm"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/runtime/vam"
	"github.com/brimdata/super/runtime/vam/op/summarize"
	"github.com/brimdata/super/runtime/vcache"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zio"
	"github.com/segmentio/ksuid"
)

var spec = &charm.Spec{
	Name:  "agg",
	Usage: "agg [flags] field[,field...] path",
	Short: "read a VNG file and run an aggregate as a test",
	Long: `
The project command reads VNG vectors from
a VNG storage objects (local files or s3 objects) and outputs
the reconstructed ZNG row data as an aggregate function.

This command is most useful for testing the vector cache and runtime.
`,
	New: newCommand,
}

func init() {
	vector.Spec.Add(spec)
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
	if len(args) != 2 {
		//XXX
		return errors.New("VNG read: must be run with a single path argument followed by one or more fields")
	}
	uri, err := storage.ParseURI(args[0])
	if err != nil {
		return err
	}
	field := args[1]
	local := storage.NewLocalEngine()
	cache := vcache.NewCache(local)
	object, err := cache.Fetch(ctx, uri, ksuid.Nil)
	if err != nil {
		return err
	}
	defer object.Close()
	//XXX nil puller
	agg := summarize.NewCountByString(zed.NewContext(), nil, field)
	writer, err := c.outputFlags.Open(ctx, local)
	if err != nil {
		return err
	}
	if err := zio.Copy(writer, zbuf.PullerReader(vam.NewMaterializer(agg))); err != nil {
		writer.Close()
		return err
	}
	return writer.Close()
}
