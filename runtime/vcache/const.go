package vcache

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
)

type const_ struct {
	meta *csup.Const
	len  uint32
}

func newConst(cctx *csup.Context, meta *csup.Const) *const_ {
	return &const_{meta: meta, len: meta.Len(cctx)}
}

func (c *const_) length() uint32 {
	return c.len
}

func (*const_) unmarshal(*csup.Context, field.Projection) {}

func (c *const_) project(loader *loader, projection field.Projection) vector.Any {
	if len(projection) > 0 {
		return vector.NewMissing(loader.sctx, c.length())
	}
	// Map the const super.Value in the csup's type context to
	// a new one in the query type context.
	val := c.meta.Value
	typ, err := loader.sctx.TranslateType(val.Type())
	if err != nil {
		panic(err)
	}
	return vector.NewConst(super.NewValue(typ, val.Bytes()), c.length())
}
