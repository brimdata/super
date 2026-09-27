package queryio

import (
	"bytes"
	"io"

	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/csup/rows"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/supio"
	"github.com/brimdata/super/sup"
)

type BSUPWriter struct {
	*csup.RowWriter
	marshaler *sup.MarshalBSUPContext
}

func NewBSUPWriter(w io.Writer) *BSUPWriter {
	m := sup.NewBSUPMarshaler()
	m.Decorate(sup.StyleSimple)
	return &BSUPWriter{
		RowWriter: csup.NewRowWriter(sio.NopCloser(w)),
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
	return w.Writer.WriteControl(buf.Bytes(), rows.ControlFormatSUP) //XXX
}
