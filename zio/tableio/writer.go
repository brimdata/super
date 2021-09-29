package tableio

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/brimdata/zed/expr"
	"github.com/brimdata/zed/zio/tzngio"
	"github.com/brimdata/zed/zng"
	"github.com/brimdata/zed/zson"
)

type Writer struct {
	writer    io.WriteCloser
	flattener *expr.Flattener
	table     *tabwriter.Writer
	typ       *zng.TypeRecord
	limit     int
	nline     int
	format    tzngio.OutFmt
}

func NewWriter(w io.WriteCloser, utf8 bool) *Writer {
	table := tabwriter.NewWriter(w, 0, 8, 1, ' ', 0)
	format := tzngio.OutFormatZeekAscii
	if utf8 {
		format = tzngio.OutFormatZeek
	}
	return &Writer{
		writer:    w,
		flattener: expr.NewFlattener(zson.NewContext()),
		table:     table,
		limit:     1000,
		format:    format,
	}
}

func (w *Writer) Write(r *zng.Record) error {
	r, err := w.flattener.Flatten(r)
	if err != nil {
		return err
	}
	if r.Type != w.typ {
		if w.typ != nil {
			w.flush()
			w.nline = 0
		}
		// First time, or new descriptor, print header
		typ := zng.TypeRecordOf(r.Type)
		w.writeHeader(typ)
		w.typ = typ
	}
	if w.nline >= w.limit {
		w.flush()
		w.writeHeader(w.typ)
		w.nline = 0
	}
	var out []string
	for k, col := range r.Columns() {
		var v string
		value := r.ValueByColumn(k)
		if col.Type == zng.TypeTime {
			if !value.IsUnsetOrNil() {
				ts, err := zng.DecodeTime(value.Bytes)
				if err != nil {
					return err
				}
				v = ts.Time().UTC().Format(time.RFC3339Nano)
			}
		} else {
			v = tzngio.FormatValue(value, w.format)
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

func (w *Writer) writeHeader(typ *zng.TypeRecord) {
	for i, c := range typ.Columns {
		if i > 0 {
			w.table.Write([]byte{'\t'})
		}
		w.table.Write([]byte(c.Name))
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
