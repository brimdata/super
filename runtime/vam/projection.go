package vam

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/vcache"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zbuf"
)

type Projection struct {
	zctx   *super.Context
	object *vcache.Object
	path   vcache.Path
}

func NewProjection(zctx *super.Context, o *vcache.Object, paths []field.Path) zbuf.Puller {
	return NewMaterializer(&Projection{
		zctx:   zctx,
		object: o,
		path:   vcache.NewProjection(paths),
	})
}

func NewVectorProjection(zctx *super.Context, o *vcache.Object, paths []field.Path) vector.Puller {
	return &Projection{
		zctx:   zctx,
		object: o,
		path:   vcache.NewProjection(paths),
	}
}

func (p *Projection) Pull(bool) (vector.Any, error) {
	if o := p.object; o != nil {
		p.object = nil
		return o.Fetch(p.zctx, p.path)
	}
	return nil, nil
}
