package bsupbytes

import (
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zio/bsupio"
)

type Deserializer struct {
	reader      *bsupio.Reader
	unmarshaler *sup.UnmarshalBSUPContext
}

func NewDeserializer(reader io.Reader, templates []interface{}) *Deserializer {
	return NewDeserializerWithContext(super.NewContext(), reader, templates)
}

func NewDeserializerWithContext(zctx *super.Context, reader io.Reader, templates []interface{}) *Deserializer {
	u := sup.NewBSUPUnmarshaler()
	u.Bind(templates...)
	return &Deserializer{
		reader:      bsupio.NewReader(zctx, reader),
		unmarshaler: u,
	}
}

func (d *Deserializer) Close() error { return d.reader.Close() }

func (d *Deserializer) Read() (interface{}, error) {
	rec, err := d.reader.Read()
	if err != nil || rec == nil {
		return nil, err
	}
	var action interface{}
	if err := d.unmarshaler.Unmarshal(*rec, &action); err != nil {
		return nil, err
	}
	return action, nil
}
