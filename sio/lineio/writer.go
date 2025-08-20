package lineio

import (
	"fmt"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sup"
)

type Writer struct {
	writer io.WriteCloser
}

func NewWriter(w io.WriteCloser) *Writer {
	return &Writer{
		writer: w,
	}
}

func (w *Writer) Close() error {
	return w.writer.Close()
}

func (w *Writer) Write(val super.Value) error {
	var s string
	if _, ok := super.TypeUnder(val.Type()).(*super.TypeOfString); ok {
		s = super.DecodeString(val.Bytes())
	} else {
		s = sup.FormatValue(val)
	}
	_, err := fmt.Fprintln(w.writer, s)
	return err
}
