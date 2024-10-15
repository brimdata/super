package zngbytes

import (
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zio/zngio"
	"github.com/brimdata/super/zson"
)

type Deserializer struct {
	reader      *zngio.Reader
	unmarshaler *zson.UnmarshalZNGContext
}

func NewDeserializer(reader io.Reader, templates []interface{}) *Deserializer {
	return NewDeserializerWithContext(zed.NewContext(), reader, templates)
}

func NewDeserializerWithContext(zctx *zed.Context, reader io.Reader, templates []interface{}) *Deserializer {
	u := zson.NewZNGUnmarshaler()
	u.Bind(templates...)
	return &Deserializer{
		reader:      zngio.NewReader(zctx, reader),
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
