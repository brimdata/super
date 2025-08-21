package cache

import (
	"context"
	"errors"
	"io"
	"path"
	"time"

	"github.com/brimdata/super/db/data"
	"github.com/brimdata/super/pkg/storage"
	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
)

type RedisCache struct {
	storage.Engine
	metrics
	client    *redis.Client
	expiry    time.Duration
	cacheable Cacheable
}

func NewRedisCache(engine storage.Engine, client *redis.Client, cacheable Cacheable, expiration time.Duration, reg prometheus.Registerer) *RedisCache {
	return &RedisCache{
		Engine:    engine,
		metrics:   newMetrics(reg),
		expiry:    expiration,
		client:    client,
		cacheable: cacheable,
	}
}

func (c *RedisCache) Get(ctx context.Context, u *storage.URI) (storage.Reader, error) {
	if !c.cacheable(u) {
		return c.Engine.Get(ctx, u)
	}
	kind, _, _ := data.FileMatch(path.Base(u.Path))
	res := c.client.Get(ctx, u.String())
	if err := res.Err(); err == nil {
		c.hits.WithLabelValues(kind.Description()).Inc()
		b, err := res.Bytes()
		if err != nil {
			return nil, err
		}
		return storage.NewBytesReader(b), nil
	} else if !errors.Is(err, redis.Nil) {
		return nil, err
	}
	reader, err := c.Engine.Get(ctx, u)
	if err != nil {
		return nil, err
	}
	// Redis values are read in the their entitety and not streamed but
	// that's okay since we only use this for smallish items like metadata
	// and small search indexes from low-cardinality sources.
	b, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	c.misses.WithLabelValues(kind.Description()).Inc()
	return storage.NewBytesReader(b), c.client.Set(ctx, u.String(), b, c.expiry).Err()
}
