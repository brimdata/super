package read

import (
	"errors"
	"flag"

	"github.com/brimdata/super"
	"github.com/brimdata/super/cli/outputflags"
	"github.com/brimdata/super/cmd/super/dev/vector"
	"github.com/brimdata/super/pkg/charm"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/runtime/vam"
	"github.com/brimdata/super/runtime/vcache"
	"github.com/brimdata/super/sbuf"
	"github.com/segmentio/ksuid"
)

var spec = &charm.Spec{
	Name:  "project",
	Usage: "project [flags] path [field ...]",
	Short: "read a CSUP file and run a projection as a test",
	Long: `
The project command reads CSUP vectors from
CSUP storage objects (local files or s3 objects) and outputs
the reconstructed BSUP row data as a projection of zero or more fields.
If no fields are specified, all the data is projected.

This command is most useful for testing the CSUP vector cache.
`,
	New: newCommand,
}

func init() {
	vector.Spec.Add(spec)
}

type Command struct {
	*vector.Command
	outputFlags outputflags.Flags
}

func newCommand(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*vector.Command)}
	c.outputFlags.SetFlags(f)
	return c, nil
}

func (c *Command) Run(args []string) error {
	ctx, cleanup, err := c.Init(&c.outputFlags)
	if err != nil {
		return err
	}
	defer cleanup()
	if len(args) < 2 {
		return errors.New("CSUP read: must be run with a single path argument followed by one or more fields")
	}
	uri, err := storage.ParseURI(args[0])
	if err != nil {
		return err
	}
	var paths []field.Path
	for _, dotted := range args[1:] {
		paths = append(paths, field.Dotted(dotted))
	}
	local := storage.NewLocalEngine()
	cache := vcache.NewCache(local)
	object, err := cache.Fetch(ctx, uri, ksuid.Nil)
	if err != nil {
		return err
	}
	defer object.Close()
	projection := vam.NewProjection(super.NewContext(), object, paths)
	writer, err := c.outputFlags.Open(ctx, local)
	if err != nil {
		return err
	}
	if err := sbuf.CopyPuller(writer, projection); err != nil {
		writer.Close()
		return err
	}
	return writer.Close()
}
