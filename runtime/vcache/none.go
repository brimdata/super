package vcache

import (
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
)

type none struct {
	meta *csup.None
}

func newNone(meta *csup.None) *none {
	return &none{meta: meta}
}

func (n *none) length() uint32 {
	return n.meta.Count
}

func (*none) unmarshal(*csup.Context, field.Projection) {}

func (n *none) project(loader *loader, projection field.Projection) vector.Any {
	if len(projection) > 0 {
		return vector.NewMissing(loader.sctx, n.length())
	}
	return vector.NewNone(n.meta.Count)
}
