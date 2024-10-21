package vector

import (
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

func (b *Bool) Type() zed.Type {
	return zed.TypeBool
}

func (b *Bool) Value(slot uint32) bool {
	// Because Bool is used to store nulls for many vectors and it is often
	// nil check to see if receiver is nil and return false.
	return b != nil && (b.Bits[slot>>6]&(1<<(slot&0x3f))) != 0
}

func (b *Bool) Set(slot uint32) {
	b.Bits[slot>>6] |= (1 << (slot & 0x3f))
}

func (b *Bool) Len() uint32 {
	return b.len
}

func (b *Bool) CopyWithBits(bits []uint64) *Bool {
	out := *b
	out.Bits = bits
	return &out
}

func (b *Bool) Serialize(builder *zcode.Builder, slot uint32) {
	if b.Nulls.Value(slot) {
		builder.Append(nil)
	} else {
		builder.Append(zed.EncodeBool(b.Value(slot)))
	}
}

func (b *Bool) AppendKey(bytes []byte, slot uint32) []byte {
	var v byte
	if b.Value(slot) {
		v = 1
	}
	return append(bytes, v)
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
	for i := range a.Len() {
		out.Bits[i] = a.Bits[i] | b.Bits[i]
	}
	return out
}

// BoolValue returns the value of slot in vec if the value is a Boolean.  It
// returns false otherwise.
func BoolValue(vec Any, slot uint32) bool {
	switch vec := Under(vec).(type) {
	case *Bool:
		return vec.Value(slot)
	case *Const:
		return vec.Value().Ptr().AsBool()
	case *Dict:
		return BoolValue(vec.Any, uint32(vec.Index[slot]))
	case *Dynamic:
		tag := vec.Tags[slot]
		return BoolValue(vec.Values[tag], vec.TagMap.Forward[slot])
	case *View:
		return BoolValue(vec.Any, vec.Index[slot])
	}
	return false
}

func NullsOf(v Any) *Bool {
	switch v := v.(type) {
	case *Array:
		return v.Nulls
	case *Bytes:
		return v.Nulls
	case *Const:
		return v.Nulls
	case *Dict:
		return v.Nulls
	case *Error:
		return Or(v.Nulls, NullsOf(v.Vals))
	case *Float:
		return v.Nulls
	case *Int:
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
		return nullsView(NullsOf(v.Any), v.Index)
	}
	panic(v)
}
