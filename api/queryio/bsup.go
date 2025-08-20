package queryio

import (
	"bytes"
	"io"

	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/bsupio"
	"github.com/brimdata/super/sio/supio"
	"github.com/brimdata/super/sup"
)

type BSUPWriter struct {
	*bsupio.Writer
	marshaler *sup.MarshalBSUPContext
}

var _ controlWriter = (*JSUPWriter)(nil)

func NewBSUPWriter(w io.Writer) *BSUPWriter {
	m := sup.NewBSUPMarshaler()
	m.Decorate(sup.StyleSimple)
	return &BSUPWriter{
		Writer:    bsupio.NewWriter(sio.NopCloser(w)),
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
	return w.Writer.WriteControl(buf.Bytes(), bsupio.ControlFormatSUP)
}
