package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type Int struct {
	Typ    super.Type
	Values []int64
}

var _ Any = (*Int)(nil)

func NewInt(typ super.Type, values []int64) *Int {
	return &Int{typ, values}
}

func NewIntEmpty(typ super.Type, length uint32) *Int {
	return NewInt(typ, make([]int64, 0, length))
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
	b.Append(super.EncodeInt(i.Values[slot]))
}

func (i *Int) Promote(typ super.Type) Promotable {
	return &Int{typ, i.Values}
}

func IntValue(vec Any, slot uint32) int64 {
	switch vec := Under(vec).(type) {
	case *Int:
		return vec.Value(slot)
	case *Const:
		return vec.val.Int()
	case *Dict:
		return IntValue(vec.Any, uint32(vec.Index[slot]))
	case *Dynamic:
		tag := vec.Tags[slot]
		return IntValue(vec.Values[tag], vec.ForwardTagMap()[slot])
	case *View:
		return IntValue(vec.Any, vec.Index[slot])
	}
	panic(vec)
}
