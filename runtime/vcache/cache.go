package vcache

import (
	"context"
	"sync"

	"github.com/brimdata/zed/pkg/storage"
	"github.com/segmentio/ksuid"
)

type Cache struct {
	mu     sync.Mutex
	engine storage.Engine
	// objects is currently a simple map but we will turn this into an
	// LRU cache sometime soon.  First step is object-level granularity, though
	// we might want LRU inside of objects based on vectors.  We can do that
	// later if measurements warrant it.  XXX note that we keep the storage
	// reader open for every object and never close it.  We should timeout
	// files and close them and then reopen them when needed to access
	// vectors that haven't yet been loaded.
	objects map[ksuid.KSUID]*Object
}

func NewCache(engine storage.Engine) *Cache {
	return &Cache{
		engine:  engine,
		objects: make(map[ksuid.KSUID]*Object),
	}
}

func (c *Cache) Fetch(ctx context.Context, uri *storage.URI, id ksuid.KSUID) (*Object, error) {
	//XXX do we want finer grained mutex?  might be ok if lookups always done inside
	// a dedicated goroutine
	c.mu.Lock()
	defer c.mu.Unlock()
	if object, ok := c.objects[id]; ok {
		return object, nil
	}
	object, err := NewObject(ctx, c.engine, uri, id)
	if err != nil {
		return nil, err
	}
	c.objects[id] = object
	return object, nil
}
