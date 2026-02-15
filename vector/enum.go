package vector

import (
	"github.com/brimdata/super"
)

type Enum struct {
	*Uint
	Typ *super.TypeEnum
}

func NewEnum(typ *super.TypeEnum, vals []uint64) *Enum {
	return &Enum{NewUint(super.TypeUint64, vals), typ}
}

func (*Enum) Kind() Kind {
	return KindEnum
}

func (e *Enum) Type() super.Type {
	return e.Typ
}
