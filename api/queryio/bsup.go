package queryio

import (
	"bytes"
	"io"

	"github.com/superdb/super"
	"github.com/superdb/super/bsup/rows"
	"github.com/superdb/super/sio"
	"github.com/superdb/super/sio/bsupio"
	"github.com/superdb/super/sio/supio"
)

type BSUPWriter struct {
	*bsupio.RowWriter
	marshaler *super.Marshaler
}

func NewBSUPWriter(w io.Writer) *BSUPWriter {
	m := super.NewMarshaler(super.NewContext())
	m.Decorate(super.StyleSimple)
	return &BSUPWriter{
		RowWriter: bsupio.NewRowWriter(sio.NopCloser(w)),
		marshaler: m,
	}
}

func (w *BSUPWriter) WriteControl(v any) error {
	val, err := w.marshaler.Marshal(v)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	err = supio.NewWriter(sio.NopCloser(&buf), supio.WriterOpts{}).Write(val)
	if err != nil {
		return err
	}
	return w.Writer.WriteControl(buf.Bytes(), rows.ControlFormatSUP) //XXX rows
}
