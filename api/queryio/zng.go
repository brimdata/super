package queryio

import (
	"bytes"
	"io"

	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/zngio"
	"github.com/brimdata/super/zio/zsonio"
	"github.com/brimdata/super/zson"
)

type ZNGWriter struct {
	*zngio.Writer
	marshaler *zson.MarshalZNGContext
}

var _ controlWriter = (*ZJSONWriter)(nil)

func NewZNGWriter(w io.Writer) *ZNGWriter {
	m := zson.NewZNGMarshaler()
	m.Decorate(zson.StyleSimple)
	return &ZNGWriter{
		Writer:    zngio.NewWriter(zio.NopCloser(w)),
		marshaler: m,
	}
}

func (w *ZNGWriter) WriteControl(v interface{}) error {
	val, err := w.marshaler.Marshal(v)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	err = zsonio.NewWriter(zio.NopCloser(&buf), zsonio.WriterOpts{}).Write(val)
	if err != nil {
		return err
	}
	return w.Writer.WriteControl(buf.Bytes(), zngio.ControlFormatZSON)
}
