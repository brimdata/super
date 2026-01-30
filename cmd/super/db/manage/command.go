package manage

import (
	"errors"
	"flag"
	"os"

	"github.com/brimdata/super/cli/dbflags"
	"github.com/brimdata/super/cli/logflags"
	"github.com/brimdata/super/cmd/super/db"
	"github.com/brimdata/super/cmd/super/db/internal/dbmanage"
	"github.com/brimdata/super/pkg/charm"
	"github.com/goccy/go-yaml"
	"go.uber.org/zap"
)

var spec = &charm.Spec{
	Name:  "manage",
	Usage: "manage",
	Short: "run compaction and other maintenance tasks on a database",
	Long: `
See https://superdb.org/command/db.html#super-db-manage
`,
	New: New,
}

func init() {
	db.Spec.Add(spec)
}

type Command struct {
	*db.Command
	logFlags logflags.Flags
	config   dbmanage.Config
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
		return yaml.UnmarshalWithOptions(b, &c.config, yaml.DisallowUnknownField())
	})
	f.Func("pool", "pool to manage (all if unset, can be specified multiple times)", func(s string) error {
		c.config.Pools = append(c.config.Pools, dbmanage.PoolConfig{Pool: s, Branch: "main"})
		return nil
	})
	c.config.Interval = f.Duration("interval", dbmanage.DefaultInterval, "interval between updates (only applicable with -monitor")
	f.BoolVar(&c.monitor, "monitor", false, "continuously monitor the database for updates")
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
	if !c.DBFlags.Quiet {
		logger, err = c.logFlags.Open()
		if err != nil {
			return err
		}
		defer logger.Sync()
	}
	if c.monitor {
		conn, err := c.DBFlags.Connection()
		if err != nil {
			if errors.Is(err, dbflags.ErrLocalDB) {
				return errors.New("monitor on local database not supported")
			}
			return err
		}
		return dbmanage.Monitor(ctx, conn, c.config, logger)
	}
	db, err := c.DBFlags.Open(ctx)
	if err != nil {
		return err
	}
	return dbmanage.Update(ctx, db, c.config, logger)
}
