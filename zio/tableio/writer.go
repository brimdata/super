package tableio

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/zio/zeekio"
	"github.com/brimdata/super/zson"
)

type Writer struct {
	writer    io.WriteCloser
	flattener *expr.Flattener
	table     *tabwriter.Writer
	typ       *super.TypeRecord
	limit     int
	nline     int
}

func NewWriter(w io.WriteCloser) *Writer {
	table := tabwriter.NewWriter(w, 0, 8, 1, ' ', 0)
	return &Writer{
		writer:    w,
		flattener: expr.NewFlattener(super.NewContext()),
		table:     table,
		limit:     1000,
	}
}

func (w *Writer) Write(r super.Value) error {
	if r.Type().Kind() != super.RecordKind {
		return fmt.Errorf("table output encountered non-record value: %s", zson.FormatValue(r))
	}
	r, err := w.flattener.Flatten(r)
	if err != nil {
		return err
	}
	if r.Type() != w.typ {
		if w.typ != nil {
			w.flush()
			w.nline = 0
		}
		// First time, or new descriptor, print header
		typ := super.TypeRecordOf(r.Type())
		w.writeHeader(typ)
		w.typ = typ
	}
	if w.nline >= w.limit {
		w.flush()
		w.writeHeader(w.typ)
		w.nline = 0
	}
	var out []string
	for k, f := range r.Fields() {
		var v string
		value := r.DerefByColumn(k).MissingAsNull()
		if f.Type == super.TypeTime {
			if !value.IsNull() {
				v = super.DecodeTime(value.Bytes()).Time().Format(time.RFC3339Nano)
			}
		} else {
			v = zeekio.FormatValue(value)
		}
		out = append(out, v)
	}
	w.nline++
	_, err = fmt.Fprintf(w.table, "%s\n", strings.Join(out, "\t"))
	return err
}

func (w *Writer) flush() error {
	return w.table.Flush()
}

func (w *Writer) writeHeader(typ *super.TypeRecord) {
	for i, f := range typ.Fields {
		if i > 0 {
			w.table.Write([]byte{'\t'})
		}
		w.table.Write([]byte(f.Name))
	}
	w.table.Write([]byte{'\n'})
}

func (w *Writer) Close() error {
	err := w.flush()
	if closeErr := w.writer.Close(); err == nil {
		err = closeErr
	}
	return err
}
