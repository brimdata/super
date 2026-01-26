package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector/bitvec"
)

type Int struct {
	Typ    super.Type
	Values []int64
	Nulls  bitvec.Bits
}

var _ Any = (*Int)(nil)
var _ Promotable = (*Int)(nil)

func NewInt(typ super.Type, values []int64, nulls bitvec.Bits) *Int {
	return &Int{Typ: typ, Values: values, Nulls: nulls}
}

func NewIntEmpty(typ super.Type, length uint32, nulls bitvec.Bits) *Int {
	return NewInt(typ, make([]int64, 0, length), nulls)
}

func (i *Int) Append(v int64) {
	i.Values = append(i.Values, v)
}

func (*Int) Kind() Kind {
	return KindInt
}

func (i *Int) Type() super.Type {
	return i.Typ
}

func (i *Int) Len() uint32 {
	return uint32(len(i.Values))
}

func (i *Int) Value(slot uint32) int64 {
	return i.Values[slot]
}

func (i *Int) Serialize(b *scode.Builder, slot uint32) {
	if i.Nulls.IsSet(slot) {
		b.Append(nil)
	} else {
		b.Append(super.EncodeInt(i.Values[slot]))
	}
}

func (i *Int) Promote(typ super.Type) Promotable {
	return &Int{typ, i.Values, i.Nulls}
}

func IntValue(vec Any, slot uint32) (int64, bool) {
	switch vec := Under(vec).(type) {
	case *Int:
		return vec.Value(slot), vec.Nulls.IsSet(slot)
	case *Const:
		return vec.val.Int(), vec.Nulls.IsSet(slot)
	case *Dict:
		if vec.Nulls.IsSet(slot) {
			return 0, true
		}
		return IntValue(vec.Any, uint32(vec.Index[slot]))
	case *Dynamic:
		tag := vec.Tags[slot]
		return IntValue(vec.Values[tag], vec.ForwardTagMap()[slot])
	case *View:
		return IntValue(vec.Any, vec.Index[slot])
	}
	panic(vec)
}
