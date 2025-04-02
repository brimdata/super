package csup

import (
	"fmt"
	"io"
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zio/bsupio"
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
	zr := bsupio.NewReader(c.local, r)
	defer zr.Close()
	for {
		val, err := zr.Read()
		if val == nil || err != nil {
			c.metas = make([]Metadata, len(c.values))
			return err
		}
		c.values = append(c.values, val.Copy())
	}
}
