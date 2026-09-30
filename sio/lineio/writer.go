package lineio

import (
	"fmt"
	"io"

	"github.com/superdb/super"
	"github.com/superdb/super/sbuf"
	"github.com/superdb/super/sup"
	"github.com/superdb/super/vector"
)

type Writer struct {
	writer io.WriteCloser
}

func NewWriter(w io.WriteCloser) *Writer {
	return &Writer{
		writer: w,
	}
}

func (w *Writer) Push(vec vector.Any) error {
	return sbuf.WriteVec(w, vec)
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
