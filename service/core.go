package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/pprof"
	"sync/atomic"

	"github.com/brimdata/zed/api"
	"github.com/brimdata/zed/lake"
	"github.com/brimdata/zed/pkg/iosrc"
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
    <p>A <a href="https://github.com/brimdata/zed/tree/main/cmd/zed/lake/serve">zqd</a> daemon is listening on this host/port.</p>
    <p>If you're a <a href="https://www.brimsecurity.com/">Brim</a> user, connect to this host/port from the <a href="https://github.com/brimdata/brim">Brim application</a> in the graphical desktop interface in your operating system (not a web browser).</p>
    <p>If your goal is to perform command line operations against this zqd, use the <a href="https://github.com/brimdata/zed/tree/main/cmd/zapi">zapi</a> client.</p>
  </body>
</html>`

type Config struct {
	Auth    AuthConfig
	Logger  *zap.Logger
	Root    string
	Version string
}

type Core struct {
	auth      *Auth0Authenticator
	conf      Config
	logger    *zap.Logger
	registry  *prometheus.Registry
	root      *lake.Root
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
	path, err := iosrc.ParseURI(conf.Root)
	if err != nil {
		return nil, err
	}
	root, err := lake.CreateOrOpen(ctx, path)
	if err != nil {
		return nil, err
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
		root:      root,
		registry:  registry,
		routerAPI: routerAPI,
		routerAux: routerAux,
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
	c.authhandle("/pool", handlePoolList).Methods("GET")
	c.authhandle("/pool", handlePoolPost).Methods("POST")
	c.authhandle("/pool/{pool}", handlePoolDelete).Methods("DELETE")
	c.authhandle("/pool/{pool}", handlePoolGet).Methods("GET")
	c.authhandle("/pool/{pool}", handlePoolPut).Methods("PUT")
	// c.authhandle("/pool/{pool}/indexsearch", handleIndexSearch).Methods("POST")
	c.authhandle("/pool/{pool}/log", handleLogStream).Methods("POST")
	c.authhandle("/pool/{pool}/log/paths", handleLogPost).Methods("POST")
	// c.authhandle("/index", handleIndexPost).Methods("POST")
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

func (c *Core) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var rm mux.RouteMatch
	if c.routerAux.Match(r, &rm) {
		rm.Handler.ServeHTTP(w, r)
		return
	}
	c.routerAPI.ServeHTTP(w, r)
}

func (c *Core) Shutdown() {
	c.logger.Info("Shutdown")
}

func (c *Core) nextTaskID() int64 {
	return atomic.AddInt64(&c.taskCount, 1)
}

func (c *Core) requestLogger(r *http.Request) *zap.Logger {
	return c.logger.With(zap.String("request_id", api.RequestIDFromContext(r.Context())))
}
