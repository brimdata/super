package queryio

import (
	"encoding/json"
	"io"
	"reflect"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/jsupio"
)

type JSUPWriter struct {
	encoder *json.Encoder
	writer  *jsupio.Writer
}

var _ controlWriter = (*JSUPWriter)(nil)

func NewJSUPWriter(w io.Writer) *JSUPWriter {
	return &JSUPWriter{
		encoder: json.NewEncoder(w),
		writer:  jsupio.NewWriter(sio.NopCloser(w)),
	}
}

func (w *JSUPWriter) Write(rec super.Value) error {
	return w.writer.Write(rec)
}

type describe struct {
	Type  string `json:"type"`
	Value any    `json:"value"`
}

func (w *JSUPWriter) WriteControl(v any) error {
	// XXX Would rather use sup.Marshal here instead of importing reflection
	// into this package, but there's an issue with marshaling nil
	// interfaces, which occurs frequently with jsupio.Object.Types. For now
	// just reflect here.
	return w.encoder.Encode(describe{
		Type:  reflect.TypeOf(v).Name(),
		Value: v,
	})
}

func (w *JSUPWriter) Close() error {
	return nil
}
