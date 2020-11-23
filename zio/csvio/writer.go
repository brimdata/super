package csvio

import (
	"encoding/csv"
	"errors"
	"io"
	"time"

	"github.com/brimsec/zq/zio/zeekio"
	"github.com/brimsec/zq/zng"
	"github.com/brimsec/zq/zng/flattener"
	"github.com/brimsec/zq/zng/resolver"
)

var ErrNotDataFrame = errors.New("csv output requires uniform records but different types encountered")

type Writer struct {
	epochDates bool
	writer     io.WriteCloser
	encoder    *csv.Writer
	flattener  *flattener.Flattener
	utf8       bool
	first      *zng.TypeRecord
}

func NewWriter(w io.WriteCloser, utf8, epochDates bool) *Writer {
	return &Writer{
		writer:     w,
		epochDates: epochDates,
		encoder:    csv.NewWriter(w),
		flattener:  flattener.New(resolver.NewContext()),
		utf8:       utf8,
	}
}

func (w *Writer) Close() error {
	w.encoder.Flush()
	return w.writer.Close()
}

func (w *Writer) Flush() error {
	w.encoder.Flush()
	return w.encoder.Error()
}

func (w *Writer) Write(rec *zng.Record) error {
	rec, err := w.flattener.Flatten(rec)
	if err != nil {
		return err
	}
	if w.first == nil {
		w.first = rec.Type
		var hdr []string
		for _, col := range rec.Type.Columns {
			hdr = append(hdr, col.Name)
		}
		if err := w.encoder.Write(hdr); err != nil {
			return err
		}
	} else if rec.Type != w.first {
		return ErrNotDataFrame
	}
	var out []string
	for k, col := range rec.Type.Columns {
		var v string
		value := rec.Value(k)
		if !w.epochDates && col.Type == zng.TypeTime {
			if !value.IsUnsetOrNil() {
				ts, err := zng.DecodeTime(value.Bytes)
				if err != nil {
					return err
				}
				v = ts.Time().UTC().Format(time.RFC3339Nano)
			}
		} else {
			v = value.Format(zng.OutFormatUnescaped)
			if !w.utf8 {
				v = zeekio.EscapeNonASCII(v)
			}
		}
		out = append(out, v)
	}
	return w.encoder.Write(out)
}
