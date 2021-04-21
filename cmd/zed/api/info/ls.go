package info

import (
	"flag"

	"github.com/brimdata/zed/api"
	"github.com/brimdata/zed/cli/outputflags"
	apicmd "github.com/brimdata/zed/cmd/zed/api"
	"github.com/brimdata/zed/pkg/charm"
	"github.com/brimdata/zed/zng"
	"github.com/brimdata/zed/zng/resolver"
	"github.com/brimdata/zed/zson"
)

var Ls = &charm.Spec{
	Name:  "ls",
	Usage: "ls [glob1 glob2 ...]",
	Short: "list spaces or information about a space",
	Long: `The ls command lists the names and information about spaces known to the system.
When run with arguments, only the spaces that match the glob-style parameters are shown
much like the traditional unix ls command.`,
	New: NewLs,
}

type LsCommand struct {
	*apicmd.Command
	lflag       bool
	outputFlags outputflags.Flags
}

func NewLs(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &LsCommand{Command: parent.(*apicmd.Command)}
	f.BoolVar(&c.lflag, "l", false, "output full information for each space")
	c.outputFlags.DefaultFormat = "text"
	c.outputFlags.SetFormatFlags(f)
	return c, nil
}

// Run lists all spaces in the current zqd host or if a parameter
// is provided (in glob style) lists the info about that space.
func (c *LsCommand) Run(args []string) error {
	ctx, cleanup, err := c.Init(&c.outputFlags)
	if err != nil {
		return err
	}
	defer cleanup()
	conn := c.Connection()
	matches, err := apicmd.SpaceGlob(ctx, conn, args...)
	if err != nil {
		if err == apicmd.ErrNoSpacesExist {
			return nil
		}
		return err
	}
	if len(matches) == 0 {
		return apicmd.ErrNoMatch
	}
	if c.lflag {
		return apicmd.WriteOutput(ctx, c.outputFlags, newSpaceReader(matches))
	}
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, m.Name)
	}
	return apicmd.WriteOutput(ctx, c.outputFlags, apicmd.NewNameReader(names))
}

type spaceReader struct {
	idx    int
	mc     *zson.MarshalZNGContext
	spaces []api.Space
}

func newSpaceReader(spaces []api.Space) *spaceReader {
	return &spaceReader{
		spaces: spaces,
		mc:     resolver.NewMarshaler(),
	}
}

func (r *spaceReader) Read() (*zng.Record, error) {
	if r.idx >= len(r.spaces) {
		return nil, nil
	}
	rec, err := r.mc.MarshalRecord(r.spaces[r.idx])
	if err != nil {
		return nil, err
	}
	r.idx++
	return rec, nil
}
