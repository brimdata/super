package vector

import (
	"math/bits"
	"slices"
	"strings"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
)

type Bool struct {
	len   uint32
	Bits  []uint64
	Nulls *Bool
}

var _ Any = (*Bool)(nil)

func NewBool(bits []uint64, len uint32, nulls *Bool) *Bool {
	return &Bool{len: len, Bits: bits, Nulls: nulls}
}

func NewBoolEmpty(length uint32, nulls *Bool) *Bool {
	return &Bool{len: length, Bits: make([]uint64, (length+63)/64), Nulls: nulls}
}

func (b *Bool) Type() super.Type {
	return super.TypeBool
}

func (b *Bool) Value(slot uint32) bool {
	// Because Bool is used to store nulls for many vectors and it is often
	// nil check to see if receiver is nil and return false.
	return b != nil && (b.Bits[slot>>6]&(1<<(slot&0x3f))) != 0
}

func (b *Bool) Set(slot uint32) {
	b.Bits[slot>>6] |= (1 << (slot & 0x3f))
}

func (b *Bool) SetLen(len uint32) {
	b.len = len
}

func (b *Bool) Len() uint32 {
	if b == nil {
		return 0
	}
	return b.len
}

func (b *Bool) CopyWithBits(bits []uint64) *Bool {
	out := *b
	out.Bits = bits
	return &out
}

func (b *Bool) Serialize(builder *zcode.Builder, slot uint32) {
	if b != nil && b.Nulls.Value(slot) {
		builder.Append(nil)
	} else {
		builder.Append(super.EncodeBool(b.Value(slot)))
	}
}

func (b *Bool) AppendKey(bytes []byte, slot uint32) []byte {
	var v byte
	if b.Value(slot) {
		v = 1
	}
	return append(bytes, v)
}

func (b *Bool) TrueCount() uint32 {
	if b == nil {
		return 0
	}
	var n uint32
	for _, bs := range b.Bits {
		n += uint32(bits.OnesCount64(bs))
	}
	return n
}

// helpful to have around for debugging
func (b *Bool) String() string {
	var s strings.Builder
	if b == nil || b.Len() == 0 {
		return "empty"
	}
	for k := uint32(0); k < b.Len(); k++ {
		if b.Value(k) {
			s.WriteByte('1')
		} else {
			s.WriteByte('0')
		}
	}
	return s.String()
}

func Not(a *Bool) *Bool {
	if a == nil {
		panic("not: nil bool")
	}
	bits := slices.Clone(a.Bits)
	for i := range bits {
		bits[i] = ^a.Bits[i]
	}
	return a.CopyWithBits(bits)
}

func Or(a, b *Bool) *Bool {
	if b == nil {
		return a
	}
	if a == nil {
		return b
	}
	if a.Len() != b.Len() {
		panic("or'ing two different length bool vectors")
	}
	out := NewBoolEmpty(a.Len(), nil)
	for i := range len(a.Bits) {
		out.Bits[i] = a.Bits[i] | b.Bits[i]
	}
	return out
}

func And(a, b *Bool) *Bool {
	if b == nil {
		return nil
	}
	if a == nil {
		return nil
	}
	if a.Len() != b.Len() {
		panic("and'ing two different length bool vectors")
	}
	out := NewBoolEmpty(a.Len(), nil)
	for i := range len(a.Bits) {
		out.Bits[i] = a.Bits[i] & b.Bits[i]
	}
	return out
}

// BoolValue returns the value of slot in vec if the value is a Boolean.  It
// returns false otherwise.
func BoolValue(vec Any, slot uint32) (bool, bool) {
	switch vec := Under(vec).(type) {
	case *Bool:
		return vec.Value(slot), vec.Nulls.Value(slot)
	case *Const:
		return vec.Value().Ptr().AsBool(), vec.Nulls.Value(slot)
	case *Dict:
		if vec.Nulls.Value(slot) {
			return false, true
		}
		return BoolValue(vec.Any, uint32(vec.Index[slot]))
	case *Dynamic:
		tag := vec.Tags[slot]
		return BoolValue(vec.Values[tag], vec.TagMap.Forward[slot])
	case *View:
		return BoolValue(vec.Any, vec.Index[slot])
	}
	panic(vec)
}

func NullsOf(v Any) *Bool {
	switch v := v.(type) {
	case *Array:
		return v.Nulls
	case *Bytes:
		return v.Nulls
	case *Bool:
		if v != nil {
			return v.Nulls
		}
		return nil
	case *Const:
		if v.Value().IsNull() {
			out := NewBoolEmpty(v.Len(), nil)
			for i := range out.Bits {
				out.Bits[i] = ^uint64(0)
			}
			return out
		}
		return v.Nulls
	case *Dict:
		return v.Nulls
	case *Enum:
		return v.Nulls
	case *Error:
		return Or(v.Nulls, NullsOf(v.Vals))
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
		return NewBoolView(NullsOf(v.Any), v.Index)
	}
	panic(v)
}

func CopyAndSetNulls(v Any, nulls *Bool) Any {
	switch v := v.(type) {
	case *Array:
		return &Array{
			Typ:     v.Typ,
			Offsets: v.Offsets,
			Values:  v.Values,
			Nulls:   nulls,
		}
	case *Bytes:
		return &Bytes{
			Offs:  v.Offs,
			Bytes: v.Bytes,
			Nulls: nulls,
		}
	case *Bool:
		return &Bool{
			len:   v.len,
			Bits:  v.Bits,
			Nulls: nulls,
		}
	case *Const:
		return &Const{
			val:   v.val,
			len:   v.len,
			Nulls: nulls,
		}
	case *Dict:
		return &Dict{
			Any:    v.Any,
			Index:  v.Index,
			Counts: v.Counts,
			Nulls:  nulls,
		}
	case *Enum:
		return &Enum{
			Typ:  v.Typ,
			Uint: CopyAndSetNulls(v.Uint, nulls).(*Uint),
		}
	case *Error:
		return &Error{
			Typ:   v.Typ,
			Vals:  v.Vals,
			Nulls: nulls,
		}
	case *Float:
		return &Float{
			Typ:    v.Typ,
			Values: v.Values,
			Nulls:  nulls,
		}
	case *Int:
		return &Int{
			Typ:    v.Typ,
			Values: v.Values,
			Nulls:  nulls,
		}
	case *IP:
		return &IP{
			Values: v.Values,
			Nulls:  nulls,
		}
	case *Map:
		return &Map{
			Typ:     v.Typ,
			Offsets: v.Offsets,
			Keys:    v.Keys,
			Values:  v.Values,
			Nulls:   nulls,
		}
	case *Named:
		return &Named{
			Typ: v.Typ,
			Any: CopyAndSetNulls(v.Any, nulls),
		}
	case *Net:
		return &Net{
			Values: v.Values,
			Nulls:  nulls,
		}
	case *Record:
		return &Record{
			Typ:    v.Typ,
			Fields: v.Fields,
			len:    v.len,
			Nulls:  nulls,
		}
	case *Set:
		return &Set{
			Typ:     v.Typ,
			Offsets: v.Offsets,
			Values:  v.Values,
			Nulls:   nulls,
		}
	case *String:
		return &String{
			Offsets: v.Offsets,
			Bytes:   v.Bytes,
			Nulls:   nulls,
		}
	case *TypeValue:
		return &TypeValue{
			Offsets: v.Offsets,
			Bytes:   v.Bytes,
			Nulls:   nulls,
		}
	case *Uint:
		return &Uint{
			Typ:    v.Typ,
			Values: v.Values,
			Nulls:  nulls,
		}
	case *Union:
		return &Union{
			Dynamic: v.Dynamic,
			Typ:     v.Typ,
			Nulls:   nulls,
		}
	default:
		panic(v)
	}
}
