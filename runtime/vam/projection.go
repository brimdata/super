package vam

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/vcache"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/vector"
)

type Projection struct {
	sctx       *super.Context
	object     *vcache.Object
	projection field.Projection
}

func NewProjection(sctx *super.Context, o *vcache.Object, paths []field.Path) sbuf.Puller {
	return NewMaterializer(&Projection{
		sctx:       sctx,
		object:     o,
		projection: field.NewProjection(paths),
	})
}

func NewVectorProjection(sctx *super.Context, o *vcache.Object, paths []field.Path) vector.Puller {
	return &Projection{
		sctx:       sctx,
		object:     o,
		projection: field.NewProjection(paths),
	}
}

func (p *Projection) Pull(bool) (vector.Any, error) {
	if o := p.object; o != nil {
		p.object = nil
		return o.Fetch(p.sctx, p.projection)
	}
	return nil, nil
}
