package csup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/sio/bsupio"
	"github.com/brimdata/super/sup"
)

type Context struct {
	mu     sync.Mutex
	local  *super.Context // holds the types for the Metadata values
	metas  []Metadata     // id to Metadata
	values []super.Value  // id to unmarshaled Metadata
	uctx   *sup.UnmarshalBSUPContext
	// On the encode path, the subtypes of all fusion types are stored
	// in a table that maps the type to a local id.  These IDs are then
	// used on the decode path via the subtypes array to map back to
	// the type value.  A future version of this logic will use a fully
	// interned typedef table like BSUP serialization does.
	subcache map[string]uint32
	smu      sync.Mutex
	subtypes []scode.Bytes
	// We store a reader to read the types on demand for the decode path.
	// If we never need the subtypes for fusion values, then they are
	// never read.  If we do read them, we read them once into the
	// subtypes table under lock smu and clear this reader value to
	// mark the table loaded.
	subtypesReader io.Reader
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

func (c *Context) lookupTypeID(sctx *super.Context, typ super.Type) uint32 {
	if c.subcache == nil {
		c.subcache = make(map[string]uint32)
	}
	// This could be more efficient by having a slice lookup on input
	// ID to output ID, but we will do that when we convert to a fully
	// interned typedef table.
	tv := sctx.LookupTypeValue(typ)
	tvBytes := tv.Bytes()
	id, ok := c.subcache[string(tvBytes)]
	if !ok {
		id = uint32(len(c.subcache))
		c.subcache[string(tvBytes)] = id
	}
	return id
}

// LookupTypeVal callable only from vcache after LoadSubtypes is called.
func (c *Context) LookupTypeVal(id uint32) scode.Bytes {
	return c.subtypes[id]
}

func (c *Context) types() []super.Value {
	if len(c.subcache) == 0 {
		return nil
	}
	types := make([]super.Value, len(c.subcache))
	for tv, id := range c.subcache {
		types[id] = super.NewValue(super.TypeType, scode.Bytes(tv))
	}
	return types
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

// LoadSubtypes is called to load the subtypes table on demand,
// only when needed.  It must be called before calling LookupTypeVal.
func (c *Context) LoadSubtypes() {
	c.smu.Lock()
	defer c.smu.Unlock()
	if c.subtypesReader != nil {
		if err := c.readTypes(c.subtypesReader); err != nil {
			// Panic for now but we should handle this more gracefully
			// when an IO error causes failure of a running query.
			panic(err)
		}
		c.subtypesReader = nil
	}
}

func (c *Context) readTypes(r io.Reader) error {
	scanner, err := bsupio.NewReader(c.local, r).NewScanner(context.TODO(), nil)
	if err != nil {
		return err
	}
	defer scanner.Pull(true)
	var vals []scode.Bytes
	for {
		batch, err := scanner.Pull(false)
		if batch == nil || err != nil {
			c.subtypes = vals
			return err
		}
		for _, val := range batch.Values() {
			if val.Type() != super.TypeType {
				return errors.New("CSUP metadata type is not a type")
			}
			vals = append(vals, val.Bytes())
		}
	}
}
