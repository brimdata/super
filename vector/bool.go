package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector/bitvec"
)

type Bool struct {
	bitvec.Bits
}

var _ Any = (*Bool)(nil)

func NewBool(bits bitvec.Bits) *Bool {
	return &Bool{bits}
}

func NewFalse(length uint32) *Bool {
	return NewBool(bitvec.NewFalse(length))
}

func NewTrue(length uint32) *Bool {
	return NewBool(bitvec.NewTrue(length))
}

func (*Bool) Kind() Kind {
	return KindBool
}

func (b *Bool) Type() super.Type {
	return super.TypeBool
}

func (b *Bool) CopyWithBits(bits bitvec.Bits) *Bool {
	out := *b
	out.Bits = bits
	return &out
}

func (b *Bool) Serialize(builder *scode.Builder, slot uint32) {
	builder.Append(super.EncodeBool(b.IsSet(slot)))
}

func Or(a, b *Bool) *Bool {
	return NewBool(bitvec.Or(a.Bits, b.Bits))
}

func Not(vec *Bool) *Bool {
	return NewBool(bitvec.Not(vec.Bits))
}

// BoolValue returns the value of slot in vec as a Boolean.
func BoolValue(vec Any, slot uint32) bool {
	switch vec := Under(vec).(type) {
	case *Bool:
		return vec.Bits.IsSet(slot)
	case *Const:
		return vec.Value().Ptr().AsBool()
	case *Dict:
		return BoolValue(vec.Any, uint32(vec.Index[slot]))
	case *Dynamic:
		tag := vec.Tags[slot]
		return BoolValue(vec.Values[tag], vec.ForwardTagMap()[slot])
	case *View:
		return BoolValue(vec.Any, vec.Index[slot])
	}
	panic(vec)
}
