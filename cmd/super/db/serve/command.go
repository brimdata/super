package serve

import (
	"context"
	"errors"
	"flag"
	"io"
	"net"
	"os"
	"runtime"
	"syscall"
	"time"

	"github.com/brimdata/super/api/client"
	"github.com/brimdata/super/cli"
	"github.com/brimdata/super/cli/logflags"
	"github.com/brimdata/super/cmd/super/db"
	"github.com/brimdata/super/cmd/super/db/internal/dbmanage"
	"github.com/brimdata/super/db/api"
	"github.com/brimdata/super/pkg/charm"
	"github.com/brimdata/super/pkg/fs"
	"github.com/brimdata/super/pkg/httpd"
	"github.com/brimdata/super/service"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

var spec = &charm.Spec{
	Name:  "serve",
	Usage: "serve [options]",
	Short: "service requests to a database",
	Long: `
The serve command implements SuperDB's server personality to service
requests from instances of SuperDB's client personality. It listens
for database API requests on the interface and port specified by
the -l option, executes the requests, and returns results.

The -log.level option controls log verbosity. Available levels,
ordered from most to least verbose, are debug, info (the default),
warn, error, dpanic, panic, and fatal. If the volume of logging
output at the default info level seems too excessive for
production use, warn level is recommended.

The -manage option enables the running of the same maintenance tasks
normally performed via the separate "zed manage" command.
`,
	HiddenFlags: "brimfd,portfile",
	New:         New,
}

func init() {
	db.Spec.Add(spec)
}

type Command struct {
	*db.Command
	conf     service.Config
	logflags logflags.Flags

	// brimfd is a file descriptor passed through by Zui desktop. If set the
	// command will exit if the fd is closed.
	brimfd          int
	listenAddr      string
	manage          time.Duration
	portFile        string
	rootContentFile string
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*db.Command)}
	c.conf.Auth.SetFlags(f)
	c.conf.Version = cli.Version()
	c.logflags.SetFlags(f)
	f.IntVar(&c.brimfd, "brimfd", -1, "pipe read fd passed by Zui to signal Zui closure")
	f.Func("cors.origin", "CORS allowed origin (may be repeated)", func(s string) error {
		c.conf.CORSAllowedOrigins = append(c.conf.CORSAllowedOrigins, s)
		return nil
	})
	f.StringVar(&c.conf.DefaultResponseFormat, "defaultfmt", service.DefaultFormat, "default response format")
	f.StringVar(&c.listenAddr, "l", ":9867", "[addr]:port to listen on")
	f.DurationVar(&c.manage, "manage", 0, "when positive, run database maintenance tasks at this interval")
	f.StringVar(&c.portFile, "portfile", "", "write listen port to file")
	f.StringVar(&c.rootContentFile, "rootcontentfile", "", "file to serve for GET /")
	return c, nil
}

func (c *Command) Run(args []string) error {
	// Don't include SIGPIPE here or else a write to a closed socket (i.e.,
	// a broken network connection) will cancel the context on Linux.
	ctx, cleanup, err := c.InitWithSignals(nil, syscall.SIGINT, syscall.SIGTERM)
	if err != nil {
		return err
	}
	defer cleanup()
	if c.conf.Root, err = c.DBFlags.URI(); err != nil {
		return err
	}
	if api.IsRemote(c.conf.Root.String()) {
		return errors.New("serve command available for local databases only")
	}
	if c.rootContentFile != "" {
		f, err := fs.Open(c.rootContentFile)
		if err != nil {
			return err
		}
		defer f.Close()
		c.conf.RootContent = f
	}
	logger, err := c.logflags.Open()
	if err != nil {
		return err
	}
	defer logger.Sync()
	if c.brimfd != -1 {
		if ctx, err = c.watchBrimFd(ctx, logger); err != nil {
			return err
		}
	}
	c.conf.Logger = logger
	core, err := service.NewCore(ctx, c.conf)
	if err != nil {
		return err
	}
	srv := httpd.New(c.listenAddr, core)
	srv.SetLogger(logger.Named("httpd"))
	if err := srv.Start(ctx); err != nil {
		return err
	}
	group, ctx := errgroup.WithContext(ctx)
	if c.manage > 0 {
		conn := client.NewConnectionTo("http://" + srv.Addr())
		group.Go(func() error {
			return dbmanage.Monitor(ctx, conn, dbmanage.Config{Interval: &c.manage}, logger.Named("manage"))
		})
	}
	if c.portFile != "" {
		if err := c.writePortFile(srv.Addr()); err != nil {
			return err
		}
	}
	group.Go(srv.Wait)
	return group.Wait()
}

func (c *Command) watchBrimFd(ctx context.Context, logger *zap.Logger) (context.Context, error) {
	if runtime.GOOS == "windows" {
		return nil, errors.New("flag -brimfd not applicable to windows")
	}
	f := os.NewFile(uintptr(c.brimfd), "brimfd")
	logger.Info("Listening to Zui process pipe", zap.String("fd", f.Name()))
	ctx, cancel := context.WithCancelCause(ctx)
	go func() {
		io.Copy(io.Discard, f)
		cancel(errors.New("Zui pipe closed"))
	}()
	return ctx, nil
}

func (c *Command) writePortFile(addr string) error {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	return fs.ReplaceFile(c.portFile, 0644, func(w io.Writer) error {
		_, err := w.Write([]byte(port))
		return err
	})
}
