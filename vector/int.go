package vector

import (
	"github.com/brimdata/zed"
	"github.com/brimdata/zed/zcode"
)

type Int struct {
	Typ    zed.Type
	Values []int64
	Nulls  *Bool
}

var _ Any = (*Int)(nil)
var _ Promotable = (*Int)(nil)

func NewInt(typ zed.Type, values []int64, nulls *Bool) *Int {
	return &Int{Typ: typ, Values: values, Nulls: nulls}
}

func NewIntEmpty(typ zed.Type, length uint32, nulls *Bool) *Int {
	return NewInt(typ, make([]int64, 0, length), nulls)
}

func (i *Int) Append(v int64) {
	i.Values = append(i.Values, v)
}

func (i *Int) Type() zed.Type {
	return i.Typ
}

func (i *Int) Len() uint32 {
	return uint32(len(i.Values))
}

func (i *Int) Value(slot uint32) int64 {
	return i.Values[slot]
}

func (i *Int) Serialize(b *zcode.Builder, slot uint32) {
	if i.Nulls.Value(slot) {
		b.Append(nil)
	} else {
		b.Append(zed.EncodeInt(i.Values[slot]))
	}
}

func (i *Int) Promote(typ zed.Type) Promotable {
	return &Int{typ, i.Values, i.Nulls}
}

func IntValue(vec Any, slot uint32) (int64, bool) {
	switch vec := Under(vec).(type) {
	case *Int:
		return vec.Value(slot), vec.Nulls.Value(slot)
	case *Const:
		return vec.val.Int(), vec.Nulls.Value(slot)
	case *Dict:
		if vec.Nulls.Value(slot) {
			return 0, true
		}
		return IntValue(vec.Any, uint32(vec.Index[slot]))
	case *Dynamic:
		tag := vec.Tags[slot]
		return IntValue(vec.Values[tag], vec.TagMap.Forward[slot])
	case *View:
		return IntValue(vec.Any, vec.Index[slot])
	}
	panic(vec)
}
