package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type Coalesce struct{}

func (*Coalesce) RipUnions() bool { return false }

func (c *Coalesce) Call(vecs ...vector.Any) vector.Any {
	for _, vec := range vecs {
		if k := vec.Kind(); k != vector.KindNull && k != vector.KindError {
			return vec
		}
	}
	return vector.NewConst(super.Null, vecs[0].Len())
}
