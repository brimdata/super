package vector

import (
	"github.com/brimdata/zed"
	"github.com/brimdata/zed/zcode"
)

type String struct {
	Offsets []uint32
	Bytes   []byte
	Nulls   *Bool
}

var _ Any = (*String)(nil)

func NewString(offsets []uint32, bytes []byte, nulls *Bool) *String {
	return &String{Offsets: offsets, Bytes: bytes, Nulls: nulls}
}

func NewStringEmpty(length uint32, nulls *Bool) *String {
	return NewString(make([]uint32, 1, length+1), nil, nulls)
}

func (s *String) Append(v string) {
	s.Bytes = append(s.Bytes, v...)
	s.Offsets = append(s.Offsets, uint32(len(s.Bytes)))
}

func (s *String) Type() zed.Type {
	return zed.TypeString
}

func (s *String) Len() uint32 {
	return uint32(len(s.Offsets) - 1)
}

func (s *String) Value(slot uint32) string {
	return string(s.Bytes[s.Offsets[slot]:s.Offsets[slot+1]])
}

func (s *String) Serialize(b *zcode.Builder, slot uint32) {
	if s.Nulls != nil && s.Nulls.Value(slot) {
		b.Append(nil)
	} else {
		b.Append(zed.EncodeString(s.Value(slot)))
	}
}
