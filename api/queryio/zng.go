package queryio

import (
	"bytes"
	"io"

	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/supio"
	"github.com/brimdata/super/zio/zngio"
)

type ZNGWriter struct {
	*zngio.Writer
	marshaler *sup.MarshalZNGContext
}

var _ controlWriter = (*ZJSONWriter)(nil)

func NewZNGWriter(w io.Writer) *ZNGWriter {
	m := sup.NewZNGMarshaler()
	m.Decorate(sup.StyleSimple)
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
	err = supio.NewWriter(zio.NopCloser(&buf), supio.WriterOpts{}).Write(val)
	if err != nil {
		return err
	}
	return w.Writer.WriteControl(buf.Bytes(), zngio.ControlFormatSUP)
}
