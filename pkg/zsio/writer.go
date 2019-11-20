package zsio

import (
	"fmt"
	"io"
	"strings"

	"github.com/mccanne/zq/pkg/zson"
	"github.com/mccanne/zq/pkg/zval"
)

type Writer struct {
	io.WriteCloser
	descriptors map[int]struct{}
}

func NewWriter(w io.WriteCloser) *Writer {
	return &Writer{
		WriteCloser: w,
		descriptors: make(map[int]struct{}),
	}
}

func (w *Writer) Write(r *zson.Record) error {
	td := r.Descriptor.ID
	_, ok := w.descriptors[td]
	if !ok {
		w.descriptors[td] = struct{}{}
		_, err := fmt.Fprintf(w.WriteCloser, "#%d:%s\n", td, r.Descriptor.Type)
		if err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w.WriteCloser, "%d:", td)
	if err != nil {
		return nil
	}
	if err := w.writeContainer(r.Raw); err != nil {
		return err
	}
	return w.write("\n")
}

func (w *Writer) write(s string) error {
	_, err := w.WriteCloser.Write([]byte(s))
	return err
}

func (w *Writer) writeContainer(val []byte) error {
	if err := w.write("["); err != nil {
		return err
	}
	if len(val) > 0 {
		for it := zval.Iter(val); !it.Done(); {
			v, container, err := it.Next()
			if err != nil {
				return err
			}
			if container {
				if err := w.writeContainer(v); err != nil {
					return err
				}
			} else {
				if err := w.writeValue(v); err != nil {
					return err
				}
			}
		}
	}
	return w.write("];")
}

func (w *Writer) writeValue(val []byte) error {
	if val == nil {
		return w.write("-;")
	}
	if err := w.write(zsonEscape(string(val))); err != nil {
		return err
	}
	return w.write(";")
}

func zsonEscape(s string) string {
	if s == "-" {
		return "\\-"
	}

	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ";", "\\;")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}
