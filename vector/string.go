package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector/bitvec"
	"github.com/brimdata/super/zcode"
)

type String struct {
	Offsets []uint32
	Bytes   []byte
	Nulls   bitvec.Bits
}

var _ Any = (*String)(nil)

func NewString(offsets []uint32, bytes []byte, nulls bitvec.Bits) *String {
	return &String{Offsets: offsets, Bytes: bytes, Nulls: nulls}
}

func NewStringEmpty(length uint32, nulls bitvec.Bits) *String {
	return NewString(make([]uint32, 1, length+1), nil, nulls)
}

func (s *String) Append(v string) {
	s.Bytes = append(s.Bytes, v...)
	s.Offsets = append(s.Offsets, uint32(len(s.Bytes)))
}

func (s *String) Type() super.Type {
	return super.TypeString
}

func (s *String) Len() uint32 {
	return uint32(len(s.Offsets) - 1)
}

func (s *String) Value(slot uint32) string {
	return string(s.Bytes[s.Offsets[slot]:s.Offsets[slot+1]])
}

func (s *String) Serialize(b *zcode.Builder, slot uint32) {
	if s.Nulls.IsSet(slot) {
		b.Append(nil)
	} else {
		b.Append(super.EncodeString(s.Value(slot)))
	}
}

func StringValue(val Any, slot uint32) (string, bool) {
	switch val := val.(type) {
	case *String:
		if val.Nulls.IsSet(slot) {
			return "", true
		}
		return val.Value(slot), false
	case *Const:
		if val.Nulls.IsSet(slot) {
			return "", true
		}
		s, _ := val.AsString()
		return s, false
	case *Dict:
		if val.Nulls.IsSet(slot) {
			return "", true
		}
		return StringValue(val.Any, uint32(val.Index[slot]))
	case *View:
		return StringValue(val.Any, val.Index[slot])
	}
	panic(val)
}
