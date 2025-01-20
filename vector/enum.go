package vector

import (
	"github.com/brimdata/super"
)

type Enum struct {
	*Uint
	Typ *super.TypeEnum
}

func NewEnum(typ *super.TypeEnum, vals []uint64, nulls *Bool) *Enum {
	return &Enum{
		Typ:  typ,
		Uint: NewUint(super.TypeUint64, vals, nulls),
	}
}

func (e *Enum) Type() super.Type { return e.Typ }
