package storage

import (
	"context"
	"fmt"
	"io"
)

type Scheme string

const (
	FileScheme  Scheme = "file"
	StdioScheme Scheme = "stdio"
	HTTPScheme  Scheme = "http"
	HTTPSScheme Scheme = "https"
	S3Scheme    Scheme = "s3"
)

// Router is an Engine that routes each function call to the correct sub-Engine
// based off the provided URI's scheme and its enablement.
type Router struct {
	engines map[Scheme]Engine
}

var _ Engine = (*Router)(nil)

func NewRouter() *Router {
	return &Router{
		engines: make(map[Scheme]Engine),
	}
}

func (r *Router) Enable(scheme Scheme) {
	var engine Engine
	switch scheme {
	case FileScheme:
		engine = NewFileSystem()
	case StdioScheme:
		engine = NewStdioEngine()
	case HTTPScheme, HTTPSScheme:
		engine = NewHTTP()
	case S3Scheme:
		engine = NewS3()
	default:
		panic(fmt.Sprintf("storage.Router.Enable: unknown scheme: %q", scheme))
	}
	r.engines[scheme] = engine
}

func (r *Router) lookup(u *URI) (Engine, error) {
	scheme := getScheme(u)
	engine, ok := r.engines[scheme]
	if !ok {
		if !knownScheme(scheme) {
			return nil, fmt.Errorf("unknown scheme %q", scheme)
		}
		return nil, fmt.Errorf("scheme %q not allowed", scheme)
	}
	return engine, nil
}

func (r *Router) Get(ctx context.Context, u *URI) (Reader, error) {
	engine, err := r.lookup(u)
	if err != nil {
		return nil, err
	}
	return engine.Get(ctx, u)
}

func (r *Router) Put(ctx context.Context, u *URI) (io.WriteCloser, error) {
	engine, err := r.lookup(u)
	if err != nil {
		return nil, err
	}
	return engine.Put(ctx, u)
}

func (r *Router) PutIfNotExists(ctx context.Context, u *URI, b []byte) error {
	engine, err := r.lookup(u)
	if err != nil {
		return err
	}
	return engine.PutIfNotExists(ctx, u, b)
}

func (r *Router) Delete(ctx context.Context, u *URI) error {
	engine, err := r.lookup(u)
	if err != nil {
		return err
	}
	return engine.Delete(ctx, u)
}

func (r *Router) DeleteByPrefix(ctx context.Context, u *URI) error {
	engine, err := r.lookup(u)
	if err != nil {
		return err
	}
	return engine.DeleteByPrefix(ctx, u)
}

func (r *Router) Size(ctx context.Context, u *URI) (int64, error) {
	engine, err := r.lookup(u)
	if err != nil {
		return 0, err
	}
	return engine.Size(ctx, u)
}

func (r *Router) Exists(ctx context.Context, u *URI) (bool, error) {
	engine, err := r.lookup(u)
	if err != nil {
		return false, err
	}
	return engine.Exists(ctx, u)
}

func (r *Router) List(ctx context.Context, u *URI) ([]Info, error) {
	engine, err := r.lookup(u)
	if err != nil {
		return nil, err
	}
	return engine.List(ctx, u)
}

func getScheme(u *URI) Scheme {
	if u.Scheme == "" {
		return FileScheme
	}
	return Scheme(u.Scheme)
}

func knownScheme(s Scheme) bool {
	switch s {
	case FileScheme, StdioScheme, HTTPScheme, HTTPSScheme, S3Scheme:
		return true
	default:
		return false
	}
}
