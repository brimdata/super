package cache

import (
	"context"
	"path"

	"github.com/brimdata/super/db/data"
	"github.com/brimdata/super/pkg/storage"
	arc "github.com/hashicorp/golang-lru/arc/v2"
	"github.com/prometheus/client_golang/prometheus"
)

type LocalCache struct {
	storage.Engine
	metrics
	lru       *arc.ARCCache[string, []byte]
	cacheable Cacheable
}

func NewLocalCache(engine storage.Engine, cacheable Cacheable, size int, registerer prometheus.Registerer) (*LocalCache, error) {
	lru, err := arc.NewARC[string, []byte](size)
	if err != nil {
		return nil, err
	}
	if registerer == nil {
		registerer = prometheus.NewRegistry()
	}
	return &LocalCache{
		Engine:    engine,
		metrics:   newMetrics(registerer),
		cacheable: cacheable,
		lru:       lru,
	}, nil
}

func (c *LocalCache) Get(ctx context.Context, u *storage.URI) (storage.Reader, error) {
	if !c.cacheable(u) {
		return c.Engine.Get(ctx, u)
	}
	kind, _, _ := data.FileMatch(path.Base(u.Path))
	if b, ok := c.lru.Get(u.String()); ok {
		c.hits.WithLabelValues(kind.Description()).Inc()
		return storage.NewBytesReader(b), nil
	}
	b, err := storage.Get(ctx, c.Engine, u)
	if err != nil {
		return nil, err
	}
	c.lru.Add(u.String(), b)
	c.misses.WithLabelValues(kind.Description()).Inc()
	return storage.NewBytesReader(b), nil
}
