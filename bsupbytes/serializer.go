package bsupbytes

import (
	"bytes"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/bsupio"
)

type Serializer struct {
	marshaler *super.Marshaler
	buffer    bytes.Buffer
	writer    *bsupio.RowWriter
}

func NewSerializer() *Serializer {
	m := super.NewMarshaler(super.NewContext())
	m.Decorate(super.StyleSimple)
	s := &Serializer{
		marshaler: m,
	}
	s.writer = bsupio.NewRowWriter(sio.NopCloser(&s.buffer))
	return s
}

func (s *Serializer) Decorate(style super.TypeStyle) {
	s.marshaler.Decorate(style)
}

func (s *Serializer) Write(v any) error {
	val, err := s.marshaler.Marshal(v)
	if err != nil {
		return err
	}
	return s.writer.Write(val)
}

// Bytes returns a slice holding the serialized values.  Close must be called
// before Bytes.
func (s *Serializer) Bytes() []byte {
	return s.buffer.Bytes()
}

func (s *Serializer) Close() error {
	return s.writer.Close()
}
