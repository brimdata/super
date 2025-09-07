package slice

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/brimdata/super"
	"github.com/brimdata/super/cli/outputflags"
	"github.com/brimdata/super/cmd/super/dev/dig"
	"github.com/brimdata/super/pkg/charm"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/bsupio"
)

var spec = &charm.Spec{
	Name:  "slice",
	Usage: "slice from:to file",
	Short: "extract a slice from a file and attempt to interpret it as BSUP",
	Long: `
The slice command takes a slice specified and a file argument (which must be a BSUP file),
extracts the requested slice of the file, and outputs the slice in any format.
The command will fail if the slice boundary does not fall on a valid BSUP boundary.`,
	New: newCommand,
}

func init() {
	dig.Spec.Add(spec)
}

type Command struct {
	*dig.Command
	outputFlags outputflags.Flags
}

func newCommand(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*dig.Command)}
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
		return errors.New("super dev dig slice: requires a from:to specifier and a file")
	}
	uri, err := storage.ParseURI(args[1])
	if err != nil {
		return err
	}
	engine := storage.NewLocalEngine()
	r, err := engine.Get(ctx, uri)
	if err != nil {
		return err
	}
	defer r.Close()
	size, err := storage.Size(r)
	if err != nil {
		return err
	}
	from, to, err := parseSlice(args[0], size)
	if err != nil {
		return err
	}
	if from == to {
		return errors.New("empty slice")
	}
	if from > to {
		return errors.New("slice start cannot be after the end")
	}
	reader := bsupio.NewReader(super.NewContext(), io.NewSectionReader(r, int64(from), int64(to-from)))
	defer reader.Close()
	writer, err := c.outputFlags.Open(ctx, engine)
	if err != nil {
		return err
	}
	if err := sio.Copy(writer, reader); err != nil {
		return err
	}
	return writer.Close()
}

func parseSlice(s string, end int64) (int, int, error) {
	vals := strings.Split(s, ":")
	if len(vals) != 2 {
		return 0, 0, errors.New("slice syntax in first argument is from:to")
	}
	from, err := strconv.Atoi(vals[0])
	if err != nil {
		return 0, 0, fmt.Errorf("slice value is not a number: %q", vals[0])
	}
	to, err := strconv.Atoi(vals[1])
	if err != nil {
		return 0, 0, fmt.Errorf("slice value is not a number: %q", vals[1])
	}
	return from, to, nil
}
