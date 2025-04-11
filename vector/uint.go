package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector/bitvec"
	"github.com/brimdata/super/zcode"
)

type Uint struct {
	loader UintLoader
	Typ    super.Type
	Nulls  bitvec.Bits
	length uint32
	values []uint64
}

var _ Any = (*Uint)(nil)
var _ Promotable = (*Uint)(nil)

func NewUint(typ super.Type, values []uint64, nulls bitvec.Bits) *Uint {
	return &Uint{Typ: typ, values: values, Nulls: nulls, length: uint32(len(values))}
}

func NewUintEmpty(typ super.Type, length uint32, nulls bitvec.Bits) *Uint {
	return NewUint(typ, make([]uint64, 0, length), nulls)
}

func NewUintWithLoader(typ super.Type, length uint32, loader UintLoader) *Uint {
	return &Uint{Typ: typ, length: length, loader: loader}
}

func (u *Uint) Append(v uint64) {
	u.values = append(u.values, v)
	u.length = uint32(len(u.values))
}

func (u *Uint) Type() super.Type {
	return u.Typ
}

func (u *Uint) Len() uint32 {
	return u.length
}

func (u *Uint) Value(slot uint32) uint64 {
	return u.Values()[slot]
}

func (u *Uint) Values() []uint64 {
	if u.values == nil {
		u.values, u.Nulls = u.loader.Load()
	}
	return u.values
}

func (u *Uint) Serialize(b *zcode.Builder, slot uint32) {
	if u.Nulls.IsSet(slot) {
		b.Append(nil)
	} else {
		b.Append(super.EncodeUint(u.Values()[slot]))
	}
}

func (u *Uint) Promote(typ super.Type) Promotable {
	copy := *u
	copy.Typ = typ
	return &copy
}

func UintValue(vec Any, slot uint32) (uint64, bool) {
	switch vec := Under(vec).(type) {
	case *Uint:
		return vec.Value(slot), vec.Nulls.IsSet(slot)
	case *Const:
		return vec.Value().Ptr().Uint(), vec.Nulls().IsSet(slot)
	case *Dict:
		if vec.Nulls().IsSet(slot) {
			return 0, true
		}
		return UintValue(vec.Any, uint32(vec.Index()[slot]))
	case *Dynamic:
		tag := vec.Tags[slot]
		return UintValue(vec.Values[tag], vec.TagMap.Forward[slot])
	case *View:
		return UintValue(vec.Any, vec.Index()[slot])
	}
	panic(vec)
}
