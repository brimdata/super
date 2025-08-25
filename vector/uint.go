package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector/bitvec"
)

type Uint struct {
	Typ    super.Type
	Values []uint64
	Nulls  bitvec.Bits
}

var _ Any = (*Uint)(nil)
var _ Promotable = (*Uint)(nil)

func NewUint(typ super.Type, values []uint64, nulls bitvec.Bits) *Uint {
	return &Uint{Typ: typ, Values: values, Nulls: nulls}
}

func NewUintEmpty(typ super.Type, length uint32, nulls bitvec.Bits) *Uint {
	return NewUint(typ, make([]uint64, 0, length), nulls)
}

func (u *Uint) Append(v uint64) {
	u.Values = append(u.Values, v)
}

func (u *Uint) Type() super.Type {
	return u.Typ
}

func (u *Uint) Len() uint32 {
	return uint32(len(u.Values))
}

func (u *Uint) Value(slot uint32) uint64 {
	return u.Values[slot]
}

func (u *Uint) Serialize(b *scode.Builder, slot uint32) {
	if u.Nulls.IsSet(slot) {
		b.Append(nil)
	} else {
		b.Append(super.EncodeUint(u.Values[slot]))
	}
}

func (u *Uint) Promote(typ super.Type) Promotable {
	return &Uint{typ, u.Values, u.Nulls}
}

func UintValue(vec Any, slot uint32) (uint64, bool) {
	switch vec := Under(vec).(type) {
	case *Uint:
		return vec.Value(slot), vec.Nulls.IsSet(slot)
	case *Const:
		return vec.Value().Ptr().Uint(), vec.Nulls.IsSet(slot)
	case *Dict:
		if vec.Nulls.IsSet(slot) {
			return 0, true
		}
		return UintValue(vec.Any, uint32(vec.Index[slot]))
	case *Dynamic:
		tag := vec.Tags[slot]
		return UintValue(vec.Values[tag], vec.ForwardTagMap()[slot])
	case *View:
		return UintValue(vec.Any, vec.Index[slot])
	}
	panic(vec)
}
