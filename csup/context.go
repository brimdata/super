package csup

import (
	"context"
	"errors"
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
	// The typedefs table is a merge of all the fusion vector subtypes.
	// Only the typedefs needed are recorded in this table and different vectors
	// are merged into this shared table by mapping each vector's IDs to the
	// shared IDs.  This is used by both the read path and write path.
	smu      sync.Mutex
	typedefs *super.TypeDefs
	// The subtypesReader holds a pointer to a reader to load the typedefs
	// bytes if they are ever needed.  If we do read them, we read them once
	// into the subtypes table under lock smu and clear this reader value to
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

func (c *Context) TypeDefs() *super.TypeDefs {
	c.smu.Lock()
	defer c.smu.Unlock()
	if c.typedefs == nil {
		c.typedefs = super.NewTypeDefs()
	}
	return c.typedefs
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

// LoadSubtypes is called to load the subtypes table on demand,
// only when needed.  It must be called before calling LookupTypeVal.
func (c *Context) LoadSubtypes() *super.TypeDefs {
	c.smu.Lock()
	defer c.smu.Unlock()
	if c.subtypesReader != nil {
		if err := c.readSubTypes(c.subtypesReader); err != nil {
			// Panic for now but we should handle this more gracefully
			// when an IO error causes failure of a running query.
			panic(err)
		}
		c.subtypesReader = nil
	}
	return c.typedefs
}

func (c *Context) readSubTypes(r io.Reader) error {
	scanner, err := bsupio.NewReader(c.local, r).NewScanner(context.TODO(), nil)
	if err != nil {
		return err
	}
	defer scanner.Pull(true)
	var vals []super.Value
	for {
		batch, err := scanner.Pull(false)
		if batch == nil || err != nil {
			if len(vals) != 1 {
				return errors.New("CSUP metadata typedefs section must be a single bytes value")
			}
			val := vals[0]
			if val.Type() != super.TypeBytes {
				return errors.New("CSUP metadata typedefs section must be a bytes type")
			}
			c.typedefs, err = super.NewTypeDefsFromBytes(val.Bytes())
			if err != nil {
				return fmt.Errorf("invalid type defs: %w", err)
			}
			return err
		}
		for _, val := range batch.Values() {
			vals = append(vals, val)
		}
	}
}
