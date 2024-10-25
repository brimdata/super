package lakeflags

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/brimdata/super/api/client"
	"github.com/brimdata/super/api/client/auth0"
	"github.com/brimdata/super/lake"
	"github.com/brimdata/super/lake/api"
	"github.com/brimdata/super/pkg/storage"
	"go.uber.org/zap"
)

var (
	ErrNoHEAD    = errors.New("HEAD not specified: indicate with -use or run the \"use\" command")
	ErrLocalLake = errors.New("cannot open connection on local lake")
)

type Flags struct {
	ConfigDir string
	Lake      string
	Quiet     bool

	lakeSpecified bool
}

func (l *Flags) SetFlags(fs *flag.FlagSet) {
	fs.BoolVar(&l.Quiet, "q", false, "quiet mode")
	dir, _ := os.UserHomeDir()
	if dir != "" {
		dir = filepath.Join(dir, ".zed")
	}
	fs.StringVar(&l.ConfigDir, "configdir", dir, "configuration and credentials directory")
	if s, ok := os.LookupEnv("SUPER_DB_LAKE"); ok {
		l.Lake, l.lakeSpecified = s, true
	}
	fs.Func("lake", fmt.Sprintf("lake location (env SUPER_DB_LAKE) (default %s)", l.Lake), func(s string) error {
		l.Lake, l.lakeSpecified = s, true
		return nil
	})
}

func (l *Flags) Connection() (*client.Connection, error) {
	uri, err := l.ClientURI()
	if err != nil {
		return nil, err
	}
	if !api.IsLakeService(uri.String()) {
		return nil, ErrLocalLake
	}
	conn := client.NewConnectionTo(uri.String())
	if err := conn.SetAuthStore(l.AuthStore()); err != nil {
		return nil, err
	}
	return conn, nil
}

func (l *Flags) Open(ctx context.Context) (api.Interface, error) {
	uri, err := l.ClientURI()
	if err != nil {
		return nil, err
	}
	if api.IsLakeService(uri.String()) {
		conn, err := l.Connection()
		if err != nil {
			return nil, err
		}
		return api.NewRemoteLake(conn), nil
	}
	lk, err := api.OpenLocalLake(ctx, zap.Must(zap.NewProduction()), uri.String())
	if errors.Is(err, lake.ErrNotExist) {
		return nil, fmt.Errorf("%w\n(hint: run 'super db init' to initialize lake at this location)", err)
	}
	return lk, err
}

func (l *Flags) AuthStore() *auth0.Store {
	return auth0.NewStore(filepath.Join(l.ConfigDir, "credentials.json"))
}

func (l *Flags) URI() (*storage.URI, error) {
	lk := strings.TrimRight(l.Lake, "/")
	if !l.lakeSpecified {
		lk = getDefaultDataDir()
	}
	if lk == "" {
		return nil, errors.New("lake location must be set (either with the -lake flag or SUPER_DB_LAKE environment variable)")
	}
	u, err := storage.ParseURI(lk)
	if err != nil {
		err = fmt.Errorf("error parsing lake location: %w", err)
	}
	return u, err
}

// ClientURI returns the URI of the lake to connect to. If the lake path is
// the defaultDataDir, it first checks if a zed service is running on
// localhost:9867 and if so uses http://localhost:9867 as the lake location.
func (l *Flags) ClientURI() (*storage.URI, error) {
	u, err := l.URI()
	if err != nil {
		return nil, err
	}
	if !l.lakeSpecified && localServer() {
		u = storage.MustParseURI("http://localhost:9867")
	}
	return u, nil
}

func localServer() bool {
	_, err := client.NewConnection().Ping(context.Background())
	return err == nil
}
