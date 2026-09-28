package vcache

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/bsup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
)

type empty struct {
	typ super.Type
}

func (*empty) length() uint32 {
	return 0
}

func newEmpty(meta *bsup.Empty) *empty {
	return &empty{typ: meta.Type}
}

func (e *empty) unmarshal(cctx *bsup.Context, projection field.Projection) {
}

func (e *empty) project(loader *loader, projection field.Projection) vector.Any {
	typ, err := loader.sctx.TranslateType(e.typ)
	if err != nil {
		panic(err)
	}
	return vector.NewEmpty(typ)
}
