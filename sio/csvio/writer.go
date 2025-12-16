package csvio

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/sup"
)

var ErrNotDataFrame = errors.New("CSV output requires uniform records but multiple types encountered (consider 'fuse')")

type Writer struct {
	writer    io.WriteCloser
	encoder   *csv.Writer
	flattener *expr.Flattener
	header    bool
	types     map[int]struct{}
	first     *super.TypeRecord
	strings   []string
}

type WriterOpts struct {
	Delim    rune
	NoHeader bool
}

func NewWriter(w io.WriteCloser, opts WriterOpts) *Writer {
	encoder := csv.NewWriter(w)
	if opts.Delim != 0 {
		encoder.Comma = opts.Delim
	}
	return &Writer{
		writer:    w,
		encoder:   encoder,
		flattener: expr.NewFlattener(super.NewContext()),
		header:    !opts.NoHeader,
		types:     make(map[int]struct{}),
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

func (w *Writer) Write(rec super.Value) error {
	if rec.Type().Kind() != super.RecordKind {
		return fmt.Errorf("CSV output encountered non-record value: %s", sup.FormatValue(rec))
	}
	rec, err := w.flattener.Flatten(rec)
	if err != nil {
		return err
	}
	if w.first == nil {
		w.first = super.TypeRecordOf(rec.Type())
		var hdr []string
		if w.header {
			for _, f := range rec.Fields() {
				hdr = append(hdr, f.Name)
			}
			if err := w.encoder.Write(hdr); err != nil {
				return err
			}
		}
	} else if _, ok := w.types[rec.Type().ID()]; !ok {
		if !fieldNamesEqual(w.first.Fields, rec.Fields()) {
			return ErrNotDataFrame
		}
		w.types[rec.Type().ID()] = struct{}{}
	}
	w.strings = w.strings[:0]
	fields := rec.Fields()
	for i, it := 0, rec.Bytes().Iter(); i < len(fields) && !it.Done(); i++ {
		var s string
		if zb := it.Next(); zb != nil {
			val := super.NewValue(fields[i].Type, zb).Under()
			switch id := val.Type().ID(); {
			case id == super.IDBytes && len(val.Bytes()) == 0:
				// We want "" instead of "0x" for a zero-length value.
			case id == super.IDString:
				s = string(val.Bytes())
			default:
				s = formatValue(val.Type(), val.Bytes())
				if super.IsFloat(id) && strings.HasSuffix(s, ".") {
					s = strings.TrimSuffix(s, ".")
				}
			}
		}
		w.strings = append(w.strings, s)
	}
	return w.encoder.Write(w.strings)
}

func formatValue(typ super.Type, bytes scode.Bytes) string {
	// Avoid SUP decoration.
	if typ.ID() < super.IDTypeComplex {
		return sup.FormatPrimitive(super.TypeUnder(typ), bytes)
	}
	return sup.FormatValue(super.NewValue(typ, bytes))
}

func fieldNamesEqual(a, b []super.Field) bool {
	return slices.EqualFunc(a, b, func(a, b super.Field) bool {
		return a.Name == b.Name
	})
}
