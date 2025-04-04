package bsupbytes

import (
	"bytes"

	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/bsupio"
)

type Serializer struct {
	marshaler *sup.MarshalBSUPContext
	buffer    bytes.Buffer
	writer    *bsupio.Writer
}

func NewSerializer() *Serializer {
	m := sup.NewBSUPMarshaler()
	m.Decorate(sup.StyleSimple)
	s := &Serializer{
		marshaler: m,
	}
	s.writer = bsupio.NewWriter(zio.NopCloser(&s.buffer))
	return s
}

func (s *Serializer) Decorate(style sup.TypeStyle) {
	s.marshaler.Decorate(style)
}

func (s *Serializer) Write(v interface{}) error {
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
