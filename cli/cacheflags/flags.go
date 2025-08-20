package cacheflags

import (
	"flag"
	"time"

	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/pkg/storage/cache"
	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
)

type Flags struct {
	Kind cache.Kind
	// LocalCacheSize specifies the number of immutable files to keep in a
	// local lru cache used to speed up searches. Values less than or equal to 0
	// (default), disables local caching of immutable files.
	LocalCacheSize int
	// RedisKeyExpiration is the expiration value used when creating keys.
	// A value of zero (meaning no expiration) should only be used when
	// Redis is configured with a key eviction policy.
	RedisKeyExpiration time.Duration
}

func (f *Flags) SetFlags(fs *flag.FlagSet) {
	fs.Var(&f.Kind, "immcache.kind", "kind of immutable cache")
	fs.IntVar(&f.LocalCacheSize, "immcache.local.size", 128, "number of small files to keep in local cache")
	fs.DurationVar(&f.RedisKeyExpiration, "immcache.redis.keyexpiry", time.Hour*24, "expiration duration of immutable keys")
}

func cacheable(u *storage.URI) bool {
	// XXX Caching was disabled when we we rewired the database.  It will
	// go back in soon, but for now this is just a stub that we can work from
	// later.  The cacheable predicate cache.Cahceable is a function that
	// says whether a URI is cacheable.  The redis cache should have a funciton
	// that is true for small things that fit into a non-streamable redis value,
	// and the forthcoming distributed SSD caching service will allow for
	// large things that are streamed.  The cache inteface is all teed up
	// so we can stack a redis cache on top of the SSD cache so all a client
	// of the cache does is access as a single top-level storage.Engine.
	return false
}

func (f *Flags) NewCache(engine storage.Engine, rclient *redis.Client, reg prometheus.Registerer) (storage.Engine, error) {
	switch f.Kind {
	case cache.KindLocal:
		return cache.NewLocalCache(engine, cacheable, f.LocalCacheSize, reg)
	case cache.KindRedis:
		return cache.NewRedisCache(engine, rclient, cacheable, f.RedisKeyExpiration, reg), nil
	}
	return nil, nil
}
