package csup

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sio/bsupio"
	"github.com/brimdata/super/sup"
)

type Context struct {
	mu     sync.Mutex
	local  *super.Context // holds the types for the Metadata values
	metas  []Metadata     // id to Metadata
	values []super.Value  // id to unmarshaled Metadata
	uctx   *sup.UnmarshalBSUPContext
}

type ID uint32

func NewContext() *Context {
	return &Context{local: super.NewContext()}
}

func (c *Context) enter(meta Metadata) ID {
	id := ID(len(c.metas))
	c.metas = append(c.metas, meta)
	return id
}

func (c *Context) Lookup(id ID) Metadata {
	if id >= ID(len(c.metas)) {
		panic(fmt.Sprintf("csup.Context ID (%d) out of range (len %d)", id, len(c.values)))
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.metas[id] == nil {
		if err := c.unmarshal(id); err != nil {
			panic(err) //XXX
		}
	}
	return c.metas[id]
}

func (c *Context) unmarshal(id ID) error {
	if c.uctx == nil {
		c.uctx = sup.NewBSUPUnmarshaler()
		c.uctx.SetContext(c.local)
		c.uctx.Bind(Template...)
	}
	if c.metas[id] != nil {
		return nil
	}
	return c.uctx.Unmarshal(c.values[id], &c.metas[id])
}

func (c *Context) readMeta(r io.Reader) error {
	scanner, err := bsupio.NewReader(c.local, r).NewScanner(context.TODO(), nil)
	if err != nil {
		return err
	}
	defer scanner.Pull(true)
	var batches []sbuf.Batch
	var numValues int
	for {
		batch, err := scanner.Pull(false)
		if err != nil {
			return err
		}
		if batch == nil {
			c.metas = make([]Metadata, numValues)
			c.values = make([]super.Value, 0, numValues)
			for _, b := range batches {
				c.values = append(c.values, b.Values()...)
			}
			return nil
		}
		batches = append(batches, batch)
		numValues += len(batch.Values())
	}
}
