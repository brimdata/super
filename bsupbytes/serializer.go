package bsupbytes

import (
	"bytes"

	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/bsupio"
	"github.com/brimdata/super/tsup"
)

type Serializer struct {
	marshaler *tsup.MarshalBSUPContext
	buffer    bytes.Buffer
	writer    *bsupio.Writer
}

func NewSerializer() *Serializer {
	m := tsup.NewBSUPMarshaler()
	m.Decorate(tsup.StyleSimple)
	s := &Serializer{
		marshaler: m,
	}
	s.writer = bsupio.NewWriter(sio.NopCloser(&s.buffer))
	return s
}

func (s *Serializer) Decorate(style tsup.TypeStyle) {
	s.marshaler.Decorate(style)
}

func (s *Serializer) Write(v any) error {
	rec, err := s.marshaler.Marshal(v)
	if err != nil {
		return err
	}
	return s.writer.Write(rec)
}

// Bytes returns a slice holding the serialized values.  Close must be called
// before Bytes.
func (s *Serializer) Bytes() []byte {
	return s.buffer.Bytes()
}

func (s *Serializer) Close() error {
	return s.writer.Close()
}
