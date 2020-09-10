package textio

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/brimsec/zq/pkg/nano"
	"github.com/brimsec/zq/zio"
	"github.com/brimsec/zq/zio/zeekio"
	"github.com/brimsec/zq/zng"
	"github.com/brimsec/zq/zng/resolver"
)

type Writer struct {
	zio.WriterFlags
	writer    io.WriteCloser
	flattener *zeekio.Flattener
	precision int
	format    zng.OutFmt
}

func NewWriter(w io.WriteCloser, flags zio.WriterFlags) *Writer {
	var format zng.OutFmt
	if flags.UTF8 {
		format = zng.OutFormatZeek
	} else {
		format = zng.OutFormatZeekAscii
	}
	return &Writer{
		writer:      w,
		WriterFlags: flags,
		flattener:   zeekio.NewFlattener(resolver.NewContext()),
		precision:   6,
		format:      format,
	}
}

func (w *Writer) Close() error {
	return w.writer.Close()
}

func (w *Writer) Write(rec *zng.Record) error {
	rec, err := w.flattener.Flatten(rec)
	if err != nil {
		return err
	}
	var out []string
	if w.ShowFields || w.ShowTypes || !w.EpochDates {
		for k, col := range rec.Type.Columns {
			var s, v string
			value := rec.Value(k)
			if !w.EpochDates && col.Name == "ts" && col.Type == zng.TypeTime {
				if value.IsUnsetOrNil() {
					v = "-"
				} else {
					ts, err := zng.DecodeTime(value.Bytes)
					if err != nil {
						return err
					}
					v = nano.Ts(ts).Time().UTC().Format(time.RFC3339Nano)
				}
			} else {
				v = value.Format(w.format)
			}
			if w.ShowFields {
				s = col.Name + ":"
			}
			if w.ShowTypes {
				s = s + col.Type.String() + ":"
			}
			out = append(out, s+v)
		}
	} else {
		var err error
		var changePrecision bool
		out, changePrecision, err = zeekio.ZeekStrings(rec, w.precision, w.format)
		if err != nil {
			return err
		}
		if changePrecision {
			w.precision = 9
		}
	}
	s := strings.Join(out, "\t")
	_, err = fmt.Fprintf(w.writer, "%s\n", s)
	return err
}
