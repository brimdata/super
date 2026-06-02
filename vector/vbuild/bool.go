package vbuild

import (
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type boolBuilder struct {
	bits bitvec.Bits
}

func (b *boolBuilder) Write(vec vector.Any) {
	if vec.Len() == 0 {
		return
	}
	switch vec := vec.(type) {
	case *vector.Const:
		v := vec.Any.(*vector.Bool).IsSet(0)
		for range vec.Len() {
			b.bits.Append(v)
		}
	case *vector.Bool:
		// There's a faster way to do this with bit shift but just go slow for
		// now.
		for i := range vec.Len() {
			b.bits.Append(vec.IsSet(i))
		}
	}
}

func (b *boolBuilder) Build() vector.Any {
	return vector.NewBool(b.bits)
}
