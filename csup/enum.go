package csup

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type EnumEncoder struct {
	*UintEncoder
	typ *super.TypeEnum
}

func NewEnumEncoder(vec *vector.Enum) *EnumEncoder {
	return &EnumEncoder{
		UintEncoder: &UintEncoder{typ: vec.Uint.Typ, vals: vec.Uint.Values},
		typ:         vec.Typ,
	}
}

func (e *EnumEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	off, values := e.UintEncoder.Metadata(cctx, off)
	return off, cctx.enter(&Enum{Symbols: e.typ.Symbols, Values: values})
}
