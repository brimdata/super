package serve

import (
	"context"
	"errors"
	"flag"
	"io"
	"net"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/brimdata/zed/cli"
	"github.com/brimdata/zed/cli/logflags"
	"github.com/brimdata/zed/cmd/zed/root"
	"github.com/brimdata/zed/lake/api"
	"github.com/brimdata/zed/pkg/charm"
	"github.com/brimdata/zed/pkg/fs"
	"github.com/brimdata/zed/pkg/httpd"
	"github.com/brimdata/zed/service"
	"go.uber.org/zap"
)

var Cmd = &charm.Spec{
	Name:  "serve",
	Usage: "serve [options]",
	Short: "service requests to a Zed lake",
	Long: `
The serve command listens for Zed lake API requests on the provided
interface and port, executes the requests, and returns results.
Requests may be issued to this service via the "zed api" command.
`,
	HiddenFlags: "brimfd,portfile",
	New:         New,
}

type Command struct {
	*root.Command
	conf     service.Config
	logflags logflags.Flags

	// brimfd is a file descriptor passed through by Zui desktop. If set the
	// command will exit if the fd is closed.
	brimfd          int
	listenAddr      string
	portFile        string
	rootContentFile string
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*root.Command)}
	c.conf.Auth.SetFlags(f)
	c.conf.Version = cli.Version()
	c.logflags.SetFlags(f)
	f.IntVar(&c.brimfd, "brimfd", -1, "pipe read fd passed by Zui to signal Zui closure")
	f.Func("cors.origin", "CORS allowed origin (may be repeated)", func(s string) error {
		c.conf.CORSAllowedOrigins = append(c.conf.CORSAllowedOrigins, s)
		return nil
	})
	f.StringVar(&c.conf.DefaultResponseFormat, "defaultfmt", service.DefaultZedFormat, "default response format")
	f.StringVar(&c.listenAddr, "l", ":9867", "[addr]:port to listen on")
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
	if c.conf.Root, err = c.LakeFlags.URI(); err != nil {
		return err
	}
	if api.IsLakeService(c.conf.Root.String()) {
		return errors.New("serve command available for local lakes only")
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
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt)
	go func() {
		sig := <-sigch
		logger.Info("Signal received", zap.Stringer("signal", sig))
	}()
	srv := httpd.New(c.listenAddr, core)
	srv.SetLogger(logger.Named("httpd"))
	if err := srv.Start(ctx); err != nil {
		return err
	}
	if c.portFile != "" {
		if err := c.writePortFile(srv.Addr()); err != nil {
			return err
		}
	}
	return srv.Wait()
}

func (c *Command) watchBrimFd(ctx context.Context, logger *zap.Logger) (context.Context, error) {
	if runtime.GOOS == "windows" {
		return nil, errors.New("flag -brimfd not applicable to windows")
	}
	f := os.NewFile(uintptr(c.brimfd), "brimfd")
	logger.Info("Listening to Zui process pipe", zap.String("fd", f.Name()))
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		io.Copy(io.Discard, f)
		logger.Info("Brim fd closed, shutting down")
		cancel()
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
