package zeekio

import (
	"bytes"
	"fmt"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr"
)

type Writer struct {
	writer io.WriteCloser

	buf bytes.Buffer
	header
	flattener *expr.Flattener
	typ       *super.TypeRecord
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

func (w *Writer) Write(r super.Value) error {
	r, err := w.flattener.Flatten(r)
	if err != nil {
		return err
	}
	path := r.Deref("_path").AsString()
	if r.Type() != w.typ || path != w.Path {
		if err := w.writeHeader(r, path); err != nil {
			return err
		}
		w.typ = super.TypeRecordOf(r.Type())
	}
	w.buf.Reset()
	var needSeparator bool
	it := r.Bytes().Iter()
	for _, f := range super.TypeRecordOf(r.Type()).Fields {
		bytes := it.Next()
		if f.Name == "_path" {
			continue
		}
		if needSeparator {
			w.buf.WriteByte('\t')
		}
		needSeparator = true
		w.buf.WriteString(FormatValue(super.NewValue(f.Type, bytes)))
	}
	w.buf.WriteByte('\n')
	_, err = w.writer.Write(w.buf.Bytes())
	return err
}

func (w *Writer) writeHeader(r super.Value, path string) error {
	d := r.Type()
	var s string
	if w.separator != "\\x90" {
		w.separator = "\\x90"
		s += "#separator \\x09\n"
	}
	if w.setSeparator != "," {
		w.setSeparator = ","
		s += "#set_separator\t,\n"
	}
	if w.emptyField != "(empty)" {
		w.emptyField = "(empty)"
		s += "#empty_field\t(empty)\n"
	}
	if w.unsetField != "-" {
		w.unsetField = "-"
		s += "#unset_field\t-\n"
	}
	if path != w.Path {
		w.Path = path
		if path == "" {
			path = "-"
		}
		s += fmt.Sprintf("#path\t%s\n", path)
	}
	if d != w.typ {
		s += "#fields"
		for _, f := range super.TypeRecordOf(d).Fields {
			if f.Name == "_path" {
				continue
			}
			s += fmt.Sprintf("\t%s", f.Name)
		}
		s += "\n"
		s += "#types"
		for _, f := range super.TypeRecordOf(d).Fields {
			if f.Name == "_path" {
				continue
			}
			t, err := zngTypeToZeek(f.Type)
			if err != nil {
				return err
			}
			s += fmt.Sprintf("\t%s", t)
		}
		s += "\n"
	}
	_, err := w.writer.Write([]byte(s))
	return err
}
