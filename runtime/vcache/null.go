package vcache

import (
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
)

type null struct {
	meta *csup.Null
}

func newNull(meta *csup.Null) *null {
	return &null{meta: meta}
}

func (n *null) length() uint32 {
	return n.meta.Count
}

func (*null) unmarshal(*csup.Context, field.Projection) {}

func (n *null) project(loader *loader, projection field.Projection) vector.Any {
	vec := vector.NewNull(n.meta.Count)
	if len(projection) > 0 {
		return vector.NewWrappedError(loader.sctx, "'.': applied to non-record", vec)
	}
	return vec
}
