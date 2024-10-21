package vector

import (
	"github.com/brimdata/super"
)

type Named struct {
	Typ *super.TypeNamed
	Any
}

var _ Any = (*Named)(nil)

func NewNamed(typ *super.TypeNamed, v Any) Any {
	return &Named{Typ: typ, Any: v}
}

func (n *Named) Type() super.Type {
	return n.Typ
}

func Under(v Any) Any {
	for {
		n, ok := v.(*Named)
		if !ok {
			return v
		}
		v = n.Any
	}
}
