package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/pprof"
	"sync"
	"sync/atomic"

	"github.com/brimdata/zed/api"
	"github.com/brimdata/zed/lake"
	"github.com/brimdata/zed/pkg/storage"
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
    <h2>zed lake serve</h2>
    <p>A <a href="https://github.com/brimdata/zed/tree/main/cmd/zed/lake/serve">zed lake service</a> is listening on this host/port.</p>
    <p>If you're a <a href="https://www.brimsecurity.com/">Brim</a> user, connect to this host/port from the <a href="https://github.com/brimdata/brim">Brim application</a> in the graphical desktop interface in your operating system (not a web browser).</p>
    <p>If your goal is to perform command line operations against this Zed lake, use the <a href="https://github.com/brimdata/zed/tree/main/cmd/zapi">zapi</a> client.</p>
  </body>
</html>`

type Config struct {
	Auth    AuthConfig
	Logger  *zap.Logger
	Root    *storage.URI
	Version string
}

type Core struct {
	auth            *Auth0Authenticator
	conf            Config
	engine          storage.Engine
	logger          *zap.Logger
	registry        *prometheus.Registry
	root            *lake.Root
	routerAPI       *mux.Router
	routerAux       *mux.Router
	taskCount       int64
	subscriptions   map[chan []byte]struct{}
	subscriptionsMu sync.RWMutex
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
	path := conf.Root
	if path == nil {
		return nil, errors.New("no lake root")
	}
	var engine storage.Engine
	switch storage.Scheme(path.Scheme) {
	case storage.FileScheme:
		engine = storage.NewLocalEngine()
	case storage.S3Scheme:
		engine = storage.NewRemoteEngine()
	default:
		return nil, fmt.Errorf("root path cannot have scheme %q", path.Scheme)
	}
	root, err := lake.CreateOrOpen(ctx, engine, path)
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
		auth:          authenticator,
		conf:          conf,
		engine:        engine,
		logger:        conf.Logger.Named("core"),
		root:          root,
		registry:      registry,
		routerAPI:     routerAPI,
		routerAux:     routerAux,
		subscriptions: make(map[chan []byte]struct{}),
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
	c.authhandle("/events", handleEvents).Methods("GET")
	c.authhandle("/pool", handlePoolList).Methods("GET")
	c.authhandle("/pool", handlePoolPost).Methods("POST")
	c.authhandle("/pool/{pool}", handlePoolDelete).Methods("DELETE")
	c.authhandle("/pool/{pool}", handlePoolGet).Methods("GET")
	c.authhandle("/pool/{pool}", handlePoolPut).Methods("PUT")
	c.authhandle("/pool/{pool}/add", handleAdd).Methods("POST")
	c.authhandle("/pool/{pool}/delete", handleDelete).Methods("POST")
	c.authhandle("/pool/{pool}/log", handleScanLog).Methods("GET")
	c.authhandle("/pool/{pool}/segments", handleScanSegments).Methods("GET")
	c.authhandle("/pool/{pool}/squash", handleSquash).Methods("POST")
	c.authhandle("/pool/{pool}/staging", handleScanStaging).Methods("GET")
	c.authhandle("/pool/{pool}/staging/{commit}", handleCommit).Methods("POST")
	c.authhandle("/pool/{pool}/stats", handlePoolStats).Methods("GET")
	c.authhandle("/query", handleQuery).Methods("POST")

	// Deprecated endpoints
	c.authhandle("/search", handleSearch).Methods("POST")
	c.authhandle("/pool/{pool}/log", handleLogPost).Methods("POST")
	c.authhandle("/pool/{pool}/log/paths", handleLogPostPaths).Methods("POST")
	// c.authhandle("/index", handleIndexPost).Methods("POST")
}

func (c *Core) handler(f func(*Core, *ResponseWriter, *Request)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res, req := newRequest(w, r, c.logger)
		f(c, res, req)
	})
}

func (c *Core) authhandle(path string, f func(*Core, *ResponseWriter, *Request)) *mux.Route {
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

func (c *Core) publishEvent(event string, data interface{}) {
	go func() {
		b, err := json.Marshal(data)
		if err != nil {
			c.logger.Error("Marshal error", zap.Error(err))
			return
		}
		payload := []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", event, b))
		c.subscriptionsMu.RLock()
		for sub := range c.subscriptions {
			sub <- payload
		}
		c.subscriptionsMu.RUnlock()
	}()
}
