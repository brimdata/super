package zqd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/pprof"
	"sync/atomic"

	"github.com/brimdata/zed/api"
	"github.com/brimdata/zed/lake/immcache"
	"github.com/brimdata/zed/pkg/iosrc"
	"github.com/brimdata/zed/ppl/zqd/apiserver"
	"github.com/brimdata/zed/ppl/zqd/db"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

const indexPage = `
<!DOCTYPE html>
<html>
  <title>ZQD daemon</title>
  <body style="padding:10px">
    <h2>ZQD</h2>
    <p>A <a href="https://github.com/brimdata/zed/tree/main/ppl/cmd/zqd">zqd</a> daemon is listening on this host/port.</p>
    <p>If you're a <a href="https://www.brimsecurity.com/">Brim</a> user, connect to this host/port from the <a href="https://github.com/brimdata/brim">Brim application</a> in the graphical desktop interface in your operating system (not a web browser).</p>
    <p>If your goal is to perform command line operations against this zqd, use the <a href="https://github.com/brimdata/zed/tree/main/cmd/zapi">zapi</a> client.</p>
  </body>
</html>`

type Config struct {
	Auth           AuthConfig
	DB             db.Config
	ImmutableCache immcache.Config
	Logger         *zap.Logger
	Redis          RedisConfig
	Root           string
	Version        string
}

type Core struct {
	auth      *Auth0Authenticator
	conf      Config
	logger    *zap.Logger
	mgr       *apiserver.Manager
	registry  *prometheus.Registry
	root      iosrc.URI
	routerAPI *mux.Router
	routerAux *mux.Router
	taskCount int64
}

func NewCore(ctx context.Context, conf Config) (*Core, error) {
	if conf.Logger == nil {
		conf.Logger = zap.NewNop()
	}
	if conf.Version == "" {
		conf.Version = "unknown"
	}

	registry := prometheus.NewRegistry()
	registry.MustRegister(prometheus.NewGoCollector())

	var authenticator *Auth0Authenticator
	if conf.Auth.Enabled {
		var err error
		if authenticator, err = NewAuthenticator(ctx, conf.Logger, registry, conf.Auth); err != nil {
			return nil, err
		}
	}

	routerAux := mux.NewRouter()
	routerAux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, indexPage)
	})

	debug := routerAux.PathPrefix("/debug/pprof").Subrouter()
	debug.HandleFunc("/cmdline", pprof.Cmdline)
	debug.HandleFunc("/profile", pprof.Profile)
	debug.HandleFunc("/symbol", pprof.Symbol)
	debug.HandleFunc("/trace", pprof.Trace)
	debug.PathPrefix("/").HandlerFunc(pprof.Index)

	routerAux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	routerAux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	})
	routerAux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&api.VersionResponse{Version: conf.Version})
	})

	routerAPI := mux.NewRouter()
	routerAPI.Use(requestIDMiddleware())
	routerAPI.Use(accessLogMiddleware(conf.Logger))
	routerAPI.Use(panicCatchMiddleware(conf.Logger))

	c := &Core{
		auth:      authenticator,
		conf:      conf,
		logger:    conf.Logger.Named("core"),
		registry:  registry,
		routerAPI: routerAPI,
		routerAux: routerAux,
	}
	if err := c.initManager(ctx); err != nil {
		c.Shutdown()
		return nil, err
	}
	c.addAPIServerRoutes()
	c.logger.Info("Started")
	return c, nil
}

func (c *Core) addAPIServerRoutes() {
	c.authhandle("/ast", handleASTPost).Methods("POST")
	c.authhandle("/auth/identity", handleAuthIdentityGet).Methods("GET")
	// /auth/method intentionally requires no authentication
	c.routerAPI.Handle("/auth/method", c.handler(handleAuthMethodGet)).Methods("GET")
	c.authhandle("/search", handleSearch).Methods("POST")
	c.authhandle("/space", handleSpaceList).Methods("GET")
	c.authhandle("/space", handleSpacePost).Methods("POST")
	c.authhandle("/space/{space}", handleSpaceDelete).Methods("DELETE")
	c.authhandle("/space/{space}", handleSpaceGet).Methods("GET")
	c.authhandle("/space/{space}", handleSpacePut).Methods("PUT")
	c.authhandle("/space/{space}/archivestat", handleArchiveStat).Methods("GET")
	c.authhandle("/space/{space}/index", handleIndexPost).Methods("POST")
	c.authhandle("/space/{space}/indexsearch", handleIndexSearch).Methods("POST")
	c.authhandle("/space/{space}/log", handleLogStream).Methods("POST")
	c.authhandle("/space/{space}/log/paths", handleLogPost).Methods("POST")
}

func (c *Core) initManager(ctx context.Context) (err error) {
	c.root, err = iosrc.ParseURI(c.conf.Root)
	if err != nil {
		return err
	}
	db, err := db.Open(ctx, c.conf.Logger, c.conf.DB, c.root)
	if err != nil {
		return err
	}
	rclient, err := NewRedisClient(ctx, c.conf.Logger, c.conf.Redis)
	if err != nil {
		return err
	}
	icache, err := immcache.New(c.conf.ImmutableCache, rclient, c.registry)
	if err != nil {
		return err
	}
	c.mgr, err = apiserver.NewManager(ctx, c.conf.Logger, nil, c.registry, c.root, db, icache)
	return err
}

func (c *Core) handler(f func(*Core, http.ResponseWriter, *http.Request)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f(c, w, r)
	})
}

func (c *Core) authhandle(path string, f func(*Core, http.ResponseWriter, *http.Request)) *mux.Route {
	var h http.Handler
	if c.auth != nil {
		h = c.auth.Middleware(c.handler(f))
	} else {
		h = c.handler(f)
	}
	return c.routerAPI.Handle(path, h)
}

func (c *Core) Registry() *prometheus.Registry {
	return c.registry
}

func (c *Core) Root() iosrc.URI {
	return c.root
}

func (c *Core) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var rm mux.RouteMatch
	if c.routerAux.Match(r, &rm) {
		rm.Handler.ServeHTTP(w, r)
		return
	}
	c.routerAPI.ServeHTTP(w, r)
}

func (c *Core) Shutdown() {
	if c.mgr != nil {
		c.mgr.Shutdown()
	}
	c.logger.Info("Shutdown")
}

func (c *Core) nextTaskID() int64 {
	return atomic.AddInt64(&c.taskCount, 1)
}

func (c *Core) requestLogger(r *http.Request) *zap.Logger {
	return c.logger.With(zap.String("request_id", api.RequestIDFromContext(r.Context())))
}
