package queryio

import (
	"bytes"
	"io"

	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/bsupio"
	"github.com/brimdata/super/zio/supio"
)

type BSUPWriter struct {
	*bsupio.Writer
	marshaler *sup.MarshalBSUPContext
}

var _ controlWriter = (*ZJSONWriter)(nil)

func NewBSUPWriter(w io.Writer) *BSUPWriter {
	m := sup.NewBSUPMarshaler()
	m.Decorate(sup.StyleSimple)
	return &BSUPWriter{
		Writer:    bsupio.NewWriter(zio.NopCloser(w)),
		marshaler: m,
	}
}

func (w *BSUPWriter) WriteControl(v any) error {
	val, err := w.marshaler.Marshal(v)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	err = supio.NewWriter(zio.NopCloser(&buf), supio.WriterOpts{}).Write(val)
	if err != nil {
		return err
	}
	return w.Writer.WriteControl(buf.Bytes(), bsupio.ControlFormatSUP)
}
