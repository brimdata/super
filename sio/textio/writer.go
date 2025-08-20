package textio

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/sio/zeekio"
)

type Writer struct {
	writer    io.WriteCloser
	flattener *expr.Flattener
}

func NewWriter(w io.WriteCloser) *Writer {
	return &Writer{
		writer:    w,
		flattener: expr.NewFlattener(super.NewContext()),
	}
}

func (w *Writer) Close() error {
	return w.writer.Close()
}

func (w *Writer) Write(val super.Value) error {
	if _, ok := super.TypeUnder(val.Type()).(*super.TypeRecord); ok {
		return w.writeRecord(val)
	}
	_, err := fmt.Fprintln(w.writer, zeekio.FormatValue(val))
	return err
}

func (w *Writer) writeRecord(rec super.Value) error {
	rec, err := w.flattener.Flatten(rec)
	if err != nil {
		return err
	}
	var out []string
	for k, f := range super.TypeRecordOf(rec.Type()).Fields {
		var s string
		value := rec.DerefByColumn(k).MissingAsNull()
		if f.Type == super.TypeTime {
			if value.IsNull() {
				s = "-"
			} else {
				s = super.DecodeTime(value.Bytes()).Time().Format(time.RFC3339Nano)
			}
		} else {
			s = zeekio.FormatValue(value)
		}
		out = append(out, s)
	}
	s := strings.Join(out, "\t")
	_, err = fmt.Fprintln(w.writer, s)
	return err
}
