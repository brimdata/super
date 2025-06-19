package manage

import (
	"bytes"
	"errors"
	"flag"
	"os"

	"github.com/brimdata/super/cli/lakeflags"
	"github.com/brimdata/super/cli/logflags"
	"github.com/brimdata/super/cmd/super/db"

	//XXX internal doesn't sound right
	"github.com/brimdata/super/cmd/super/internal/lakemanage"
	"github.com/brimdata/super/pkg/charm"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

var spec = &charm.Spec{
	Name:  "manage",
	Usage: "manage",
	Short: "run compaction and other maintenance tasks on a lake",
	Long: `
The manage command performs maintenance tasks on a lake.

Currently the only supported task is compaction, which reduces
fragmentation by reading data objects in a pool and writing their
contents back to large, non-overlapping objects.

If the -monitor option is specified and the lake is located via network
connection, zed manage will run continuously and perform updates as
needed. By default a check is performed once per minute to determine if
updates are necessary. The -interval option may be used to specify an
alternate check frequency in duration format.

If -monitor is not specified, a single maintenance pass is performed on
the lake.

The output from manage provides a per-pool summary of the maintenance
performed, including a count of objects_compacted.

As an alternative to running manage as a separate command, the -manage
option is also available on the "zed serve" command to have maintenance
tasks run at the specified interval by the service process.
`,
	New: New,
}

func init() {
	db.Spec.Add(spec)
}

type Command struct {
	*db.Command
	logFlags logflags.Flags
	config   lakemanage.Config
	monitor  bool
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*db.Command)}
	c.logFlags.SetFlags(f)
	f.Func("config", "path of manage YAML config file", func(s string) error {
		b, err := os.ReadFile(s)
		if err != nil {
			return err
		}
		d := yaml.NewDecoder(bytes.NewReader(b))
		d.KnownFields(true) // returns error for unknown fields
		return d.Decode(&c.config)
	})
	f.Func("pool", "pool to manage (all if unset, can be specified multiple times)", func(s string) error {
		c.config.Pools = append(c.config.Pools, lakemanage.PoolConfig{Pool: s, Branch: "main"})
		return nil
	})
	c.config.Interval = f.Duration("interval", lakemanage.DefaultInterval, "interval between updates (only applicable with -monitor")
	f.BoolVar(&c.monitor, "monitor", false, "continuously monitor the lake for updates")
	f.BoolVar(&c.config.Vectors, "vectors", false, "create vectors for objects")
	return c, nil
}

func (c *Command) Run(args []string) error {
	ctx, cleanup, err := c.Init()
	if err != nil {
		return err
	}
	defer cleanup()
	logger := zap.NewNop()
	if !c.LakeFlags.Quiet {
		logger, err = c.logFlags.Open()
		if err != nil {
			return err
		}
		defer logger.Sync()
	}
	if c.monitor {
		conn, err := c.LakeFlags.Connection()
		if err != nil {
			if errors.Is(err, lakeflags.ErrLocalDB) {
				return errors.New("monitor on local database not supported")
			}
			return err
		}
		return lakemanage.Monitor(ctx, conn, c.config, logger)
	}
	lk, err := c.LakeFlags.Open(ctx)
	if err != nil {
		return err
	}
	return lakemanage.Update(ctx, lk, c.config, logger)
}
