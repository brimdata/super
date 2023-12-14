package vector

import (
	"github.com/brimdata/zed"
	"github.com/brimdata/zed/zcode"
)

type Bool struct {
	mem
	Typ    zed.Type
	Values []bool //XXX bit vector
}

var _ Any = (*Bool)(nil)

func NewBool(typ zed.Type, vals []bool) *Bool {
	return &Bool{Typ: typ, Values: vals}
}

func (b *Bool) Type() zed.Type {
	return b.Typ
}

func (b *Bool) NewBuilder() Builder {
	vals := b.Values
	var voff int
	return func(b *zcode.Builder) bool {
		if voff < len(vals) {
			b.Append(zed.EncodeBool(vals[voff]))
			voff++
			return true

		}
		return false
	}
}

func (b *Bool) Key(bytes []byte, slot int) []byte {
	var v byte
	if b.Values[slot] {
		v = 1
	}
	return append(bytes, v)
}

func (b *Bool) Length() int {
	return len(b.Values)
}

func (b *Bool) Serialize(slot int) *zed.Value {
	return zed.NewBool(b.Values[slot])
}
