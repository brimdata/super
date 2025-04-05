package csup

import "github.com/brimdata/super"

type Context struct {
	local     *super.Context // holds the types for the Metadata values
	metas     []Metadata
	seralized []super.Value
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
	return c.metas[id]
}
