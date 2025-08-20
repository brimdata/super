package bsupbytes

import (
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sio/bsupio"
	"github.com/brimdata/super/sup"
)

type Deserializer struct {
	reader      *bsupio.Reader
	unmarshaler *sup.UnmarshalBSUPContext
}

func NewDeserializer(reader io.Reader, templates []any) *Deserializer {
	return NewDeserializerWithContext(super.NewContext(), reader, templates)
}

func NewDeserializerWithContext(sctx *super.Context, reader io.Reader, templates []any) *Deserializer {
	u := sup.NewBSUPUnmarshaler()
	u.Bind(templates...)
	return &Deserializer{
		reader:      bsupio.NewReader(sctx, reader),
		unmarshaler: u,
	}
}

func (d *Deserializer) Close() error { return d.reader.Close() }

func (d *Deserializer) Read() (any, error) {
	rec, err := d.reader.Read()
	if err != nil || rec == nil {
		return nil, err
	}
	var action any
	if err := d.unmarshaler.Unmarshal(*rec, &action); err != nil {
		return nil, err
	}
	return action, nil
}
