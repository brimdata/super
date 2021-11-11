package api

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"

	"github.com/brimdata/zed/api/client"
	zedlake "github.com/brimdata/zed/cmd/zed/lake"
	"github.com/brimdata/zed/cmd/zed/root"
	"github.com/brimdata/zed/lake/api"
	"github.com/brimdata/zed/pkg/charm"
)

var Cmd = &charm.Spec{
	Name:  "api",
	Usage: "api [options] sub-command",
	Short: "perform lake actions on Zed service",
	Long: `
The "api" command provides client access to a Zed lake service running
on the IP and port provided in the "-host" option.  This option defaults
to localhost:9867 so you can conveniently connect to a lake service
running locally on the default port, as is automatically launched
by the Brim application for the "local Zed lake".  If the port is ommitted
from the host string, then 9867 is assumed.

You can also set the environment variable ZED_LAKE_HOST to override the default
"-host" option of localhost:9867.

All of the relevant "lake" commands are available through the "api" command.
Refer to the help of the individual sub-commands for more details.`,
	New: New,
}

type Command struct {
	*root.Command
	Host      string
	configDir string
}

var _ zedlake.Command = (*Command)(nil)

const HostEnv = "ZED_LAKE_HOST"

func DefaultHost() string {
	host := os.Getenv(HostEnv)
	if host == "" {
		host = "localhost:9867"
	}
	return host
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	dir, _ := os.UserHomeDir()
	if dir != "" {
		dir = filepath.Join(dir, ".zed")
	}
	c := &Command{Command: parent.(*root.Command)}
	f.StringVar(&c.Host, "host", DefaultHost(), "host[:port] of Zed lake service")
	f.StringVar(&c.configDir, "configdir", dir, "configuration and credentials directory")
	return c, nil
}

func (c *Command) Root() *root.Command {
	return c.Command
}

func (c *Command) Connection() (*client.Connection, error) {
	creds, err := c.LoadCredentials()
	if err != nil {
		return nil, err
	}
	host := c.Host
	if !strings.HasPrefix(host, "http") {
		host = "http://" + host
	}
	conn := client.NewConnectionTo(host)
	if token, ok := creds.ServiceTokens(c.Host); ok {
		conn.SetAuthToken(token.Access)
	}
	return conn, nil
}

func (c *Command) Open(ctx context.Context) (api.Interface, error) {
	conn, err := c.Connection()
	if err != nil {
		return nil, err
	}
	return api.NewRemoteWithConnection(conn), nil
}
