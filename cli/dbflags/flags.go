package dbflags

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
	"github.com/brimdata/super/db"
	"github.com/brimdata/super/db/api"
	"github.com/brimdata/super/pkg/storage"
	"go.uber.org/zap"
)

var (
	ErrNoHEAD  = errors.New("HEAD not specified: indicate with -use or run the \"use\" command")
	ErrLocalDB = errors.New("cannot open connection on local database")
)

type Flags struct {
	ConfigDir string
	DB        string
	Quiet     bool

	dbSpecified bool
}

func (l *Flags) SetFlags(fs *flag.FlagSet) {
	fs.BoolVar(&l.Quiet, "q", false, "quiet mode")
	dir, _ := os.UserHomeDir()
	if dir != "" {
		dir = filepath.Join(dir, ".super")
	}
	fs.StringVar(&l.ConfigDir, "configdir", dir, "configuration and credentials directory")
	if s, ok := os.LookupEnv("SUPER_DB"); ok {
		l.DB, l.dbSpecified = s, true
	}
	fs.Func("db", fmt.Sprintf("database location (env SUPER_DB) (default %s)", l.DB), func(s string) error {
		l.DB, l.dbSpecified = s, true
		return nil
	})
}

func (l *Flags) Connection() (*client.Connection, error) {
	uri, err := l.ClientURI()
	if err != nil {
		return nil, err
	}
	if !api.IsRemote(uri.String()) {
		return nil, ErrLocalDB
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
	if api.IsRemote(uri.String()) {
		conn, err := l.Connection()
		if err != nil {
			return nil, err
		}
		return api.NewRemoteDB(conn), nil
	}
	DB, err := api.Connect(ctx, zap.Must(zap.NewProduction()), uri.String())
	if errors.Is(err, db.ErrNotExist) {
		return nil, fmt.Errorf("%w\n(hint: run 'super db init' to initialize a database at this location)", err)
	}
	return DB, err
}

func (l *Flags) AuthStore() *auth0.Store {
	return auth0.NewStore(filepath.Join(l.ConfigDir, "credentials.json"))
}

func (l *Flags) URI() (*storage.URI, error) {
	db := strings.TrimRight(l.DB, "/")
	if !l.dbSpecified {
		db = getDefaultDataDir()
	}
	if db == "" {
		return nil, errors.New("database location must be set (either with the -db flag or SUPER_DB environment variable)")
	}
	u, err := storage.ParseURI(db)
	if err != nil {
		err = fmt.Errorf("error parsing database location: %w", err)
	}
	return u, err
}

// ClientURI returns the URI of the database to connect to. If the database path is
// the defaultDataDir, it first checks if a SuperDB service is running on
// localhost:9867 and if so uses http://localhost:9867 as the database location.
func (l *Flags) ClientURI() (*storage.URI, error) {
	u, err := l.URI()
	if err != nil {
		return nil, err
	}
	if !l.dbSpecified && localServer() {
		u = storage.MustParseURI("http://localhost:9867")
	}
	return u, nil
}

func localServer() bool {
	_, err := client.NewConnection().Ping(context.Background())
	return err == nil
}
