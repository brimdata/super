package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
)

type Uint struct {
	Typ    zed.Type
	Values []uint64
	Nulls  *Bool
}

var _ Any = (*Uint)(nil)
var _ Promotable = (*Uint)(nil)

func NewUint(typ zed.Type, values []uint64, nulls *Bool) *Uint {
	return &Uint{Typ: typ, Values: values, Nulls: nulls}
}

func NewUintEmpty(typ zed.Type, length uint32, nulls *Bool) *Uint {
	return NewUint(typ, make([]uint64, 0, length), nulls)
}

func (u *Uint) Append(v uint64) {
	u.Values = append(u.Values, v)
}

func (u *Uint) Type() zed.Type {
	return u.Typ
}

func (u *Uint) Len() uint32 {
	return uint32(len(u.Values))
}

func (u *Uint) Value(slot uint32) uint64 {
	return u.Values[slot]
}

func (u *Uint) Serialize(b *zcode.Builder, slot uint32) {
	if u.Nulls.Value(slot) {
		b.Append(nil)
	} else {
		b.Append(zed.EncodeUint(u.Values[slot]))
	}
}

func (u *Uint) AppendKey(b []byte, slot uint32) []byte {
	if u.Nulls.Value(slot) {
		b = append(b, 0)
	}
	val := u.Values[slot]
	b = append(b, byte(val>>(8*7)))
	b = append(b, byte(val>>(8*6)))
	b = append(b, byte(val>>(8*5)))
	b = append(b, byte(val>>(8*4)))
	b = append(b, byte(val>>(8*3)))
	b = append(b, byte(val>>(8*2)))
	b = append(b, byte(val>>(8*1)))
	return append(b, byte(val>>(8*0)))
}

func (u *Uint) Promote(typ zed.Type) Promotable {
	return &Uint{typ, u.Values, u.Nulls}
}

func UintValue(vec Any, slot uint32) (uint64, bool) {
	switch vec := Under(vec).(type) {
	case *Uint:
		return vec.Value(slot), vec.Nulls.Value(slot)
	case *Const:
		return vec.Value().Ptr().Uint(), vec.Nulls.Value(slot)
	case *Dict:
		return UintValue(vec.Any, uint32(vec.Index[slot]))
	case *Dynamic:
		tag := vec.Tags[slot]
		return UintValue(vec.Values[tag], vec.TagMap.Forward[slot])
	case *View:
		return UintValue(vec.Any, vec.Index[slot])
	}
	panic(vec)
}
