package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector/bitvec"
	"github.com/brimdata/super/zcode"
)

type Bool struct {
	bitvec.Bits
	Nulls bitvec.Bits
}

var _ Any = (*Bool)(nil)

func NewBool(bits bitvec.Bits, nulls bitvec.Bits) *Bool {
	return &Bool{Bits: bits, Nulls: nulls}
}

func NewBoolEmpty(length uint32, nulls bitvec.Bits) *Bool {
	return &Bool{Bits: bitvec.NewFalse(length), Nulls: nulls}
}

func NewBoolEmpty3(length uint32, nulls bitvec.Bits) *Bool {
	return &Bool{Bits: bitvec.NewFalse(length), Nulls: nulls}
}

func NewFalse(length uint32) *Bool {
	return NewBoolEmpty(length, bitvec.Zero)
}

func NewTrue(length uint32) *Bool {
	return NewBool(bitvec.NewTrue(length), bitvec.Zero)
}

func (b *Bool) Type() super.Type {
	return super.TypeBool
}

func (b *Bool) CopyWithBits(bits bitvec.Bits) *Bool {
	out := *b
	out.Bits = bits
	return &out
}

func (b *Bool) Serialize(builder *zcode.Builder, slot uint32) {
	if b.Nulls.IsSet(slot) {
		builder.Append(nil)
	} else {
		builder.Append(super.EncodeBool(b.IsSet(slot)))
	}
}

// Or is a simple case of logical-or where we don't care about nulls in
// the input (presuming the corresponding bits to be false) and we return
// the or'd result as a boolean vector without nulls.
func Or(a, b *Bool) *Bool {
	return NewBool(bitvec.Or(a.Bits, b.Bits), bitvec.Zero)
}

// BoolValue returns the value of slot in vec if the value is a Boolean.  It
// returns false otherwise.
func BoolValue(vec Any, slot uint32) (bool, bool) {
	switch vec := Under(vec).(type) {
	case *Bool:
		return vec.Bits.IsSet(slot), vec.Nulls.IsSet(slot)
	case *Const:
		return vec.Value().Ptr().AsBool(), vec.Nulls.IsSet(slot)
	case *Dict:
		if vec.Nulls.IsSet(slot) {
			return false, true
		}
		return BoolValue(vec.Any, uint32(vec.Index[slot]))
	case *Dynamic:
		tag := vec.Tags[slot]
		return BoolValue(vec.Values[tag], vec.ForwardTagMap()[slot])
	case *View:
		return BoolValue(vec.Any, vec.Index[slot])
	}
	panic(vec)
}

func NullsOf(v Any) bitvec.Bits {
	switch v := v.(type) {
	case *Array:
		return v.Nulls
	case *Bytes:
		return v.Nulls
	case *Bool:
		return v.Nulls
	case *Const:
		if v.Value().IsNull() {
			return bitvec.NewTrue(v.Len())
		}
		return v.Nulls
	case *Dict:
		return v.Nulls
	case *Enum:
		return v.Nulls
	case *Error:
		return bitvec.Or(v.Nulls, NullsOf(v.Vals))
	case *Float:
		return v.Nulls
	case *Int:
		return v.Nulls
	case *IP:
		return v.Nulls
	case *Map:
		return v.Nulls
	case *Named:
		return NullsOf(v.Any)
	case *Net:
		return v.Nulls
	case *Record:
		return v.Nulls
	case *Set:
		return v.Nulls
	case *String:
		return v.Nulls
	case *TypeValue:
		return v.Nulls
	case *Uint:
		return v.Nulls
	case *Union:
		return v.Nulls
	case *View:
		return NullsOf(v.Any).Pick(v.Index)
	}
	panic(v)
}

func CopyAndSetNulls(v Any, nulls bitvec.Bits) Any {
	switch v := v.(type) {
	case *Array:
		copy := *v
		copy.Nulls = nulls
		return &copy
	case *Bytes:
		copy := *v
		copy.Nulls = nulls
		return &copy
	case *Bool:
		copy := *v
		copy.Nulls = nulls
		return &copy
	case *Const:
		copy := *v
		copy.Nulls = nulls
		return &copy
	case *Dict:
		copy := *v
		copy.Nulls = nulls
		return &copy
	case *Enum:
		return &Enum{
			Typ:  v.Typ,
			Uint: CopyAndSetNulls(v.Uint, nulls).(*Uint),
		}
	case *Error:
		copy := *v
		copy.Nulls = nulls
		return &copy
	case *Float:
		copy := *v
		copy.Nulls = nulls
		return &copy
	case *Int:
		copy := *v
		copy.Nulls = nulls
		return &copy
	case *IP:
		copy := *v
		copy.Nulls = nulls
		return &copy
	case *Map:
		copy := *v
		copy.Nulls = nulls
		return &copy
	case *Named:
		return &Named{
			Typ: v.Typ,
			Any: CopyAndSetNulls(v.Any, nulls),
		}
	case *Net:
		copy := *v
		copy.Nulls = nulls
		return &copy
	case *Record:
		copy := *v
		copy.Nulls = nulls
		return &copy
	case *Set:
		copy := *v
		copy.Nulls = nulls
		return &copy
	case *String:
		copy := *v
		copy.Nulls = nulls
		return &copy
	case *TypeValue:
		copy := *v
		copy.Nulls = nulls
		return &copy
	case *Uint:
		copy := *v
		copy.Nulls = nulls
		return &copy
	case *Union:
		return NewUnion(v.Typ, v.Tags, v.Values, nulls)
	default:
		panic(v)
	}
}
