package query

import (
	"flag"
	"os"

	"github.com/brimdata/super/cli/outputflags"
	"github.com/brimdata/super/cli/poolflags"
	"github.com/brimdata/super/cli/queryflags"
	"github.com/brimdata/super/cli/runtimeflags"
	"github.com/brimdata/super/cmd/super/db"
	"github.com/brimdata/super/pkg/charm"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/zsonio"
)

var spec = &charm.Spec{
	Name:  "query",
	Usage: "query [options] [zed-query]",
	Short: "run a query on a Zed data lake",
	Long: `
"zed query" runs a query on a Zed data lake.
`,
	New: New,
}

func init() {
	db.Spec.Add(spec)
}

type Command struct {
	*db.Command
	outputFlags  outputflags.Flags
	poolFlags    poolflags.Flags
	queryFlags   queryflags.Flags
	runtimeFlags runtimeflags.Flags
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*db.Command)}
	c.outputFlags.SetFlags(f)
	c.poolFlags.SetFlags(f)
	c.queryFlags.SetFlags(f)
	c.runtimeFlags.SetFlags(f)
	return c, nil
}

func (c *Command) Run(args []string) error {
	ctx, cleanup, err := c.Init(&c.outputFlags, &c.runtimeFlags)
	if err != nil {
		return err
	}
	defer cleanup()
	if len(args) > 1 || len(args) == 0 && len(c.queryFlags.Includes) == 0 {
		return charm.NeedHelp
	}
	var src string
	if len(args) == 1 {
		src = args[0]
	}
	lake, err := c.LakeFlags.Open(ctx)
	if err != nil {
		return err
	}
	w, err := c.outputFlags.Open(ctx, storage.NewLocalEngine())
	if err != nil {
		return err
	}
	head, _ := c.poolFlags.HEAD()
	query, err := lake.Query(ctx, head, c.queryFlags.SQL, src, c.queryFlags.Includes...)
	if err != nil {
		w.Close()
		return err
	}
	defer query.Pull(true)
	out := map[string]zio.WriteCloser{
		"main":  w,
		"debug": zsonio.NewWriter(zio.NopCloser(os.Stderr), zsonio.WriterOpts{}),
	}
	err = zbuf.CopyMux(out, query)
	if closeErr := w.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		c.queryFlags.PrintStats(query.Progress())
	}
	return err
}
