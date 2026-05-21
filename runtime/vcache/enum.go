package vcache

import (
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
)

type enum struct {
	meta   *csup.Enum
	values shadow
}

func (e *enum) length() uint32 {
	return e.values.length()
}

func newEnum(meta *csup.Enum, values shadow) *enum {
	return &enum{
		meta:   meta,
		values: values,
	}
}

func (e *enum) unmarshal(cctx *csup.Context, projection field.Projection) {
	e.values.unmarshal(cctx, projection)
}

func (e *enum) project(loader *loader, projection field.Projection) vector.Any {
	vec := e.values.project(loader, projection).(*vector.Uint)
	enum := loader.sctx.LookupTypeEnum(e.meta.Symbols)
	return &vector.Enum{Uint: vec, Typ: enum}
}
