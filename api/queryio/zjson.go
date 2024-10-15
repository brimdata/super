package queryio

import (
	"encoding/json"
	"io"
	"reflect"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/zjsonio"
)

type ZJSONWriter struct {
	encoder *json.Encoder
	writer  *zjsonio.Writer
}

var _ controlWriter = (*ZJSONWriter)(nil)

func NewZJSONWriter(w io.Writer) *ZJSONWriter {
	return &ZJSONWriter{
		encoder: json.NewEncoder(w),
		writer:  zjsonio.NewWriter(zio.NopCloser(w)),
	}
}

func (w *ZJSONWriter) Write(rec zed.Value) error {
	return w.writer.Write(rec)
}

type describe struct {
	Type  string      `json:"type"`
	Value interface{} `json:"value"`
}

func (w *ZJSONWriter) WriteControl(v interface{}) error {
	// XXX Would rather use zson Marshal here instead of importing reflection
	// into this package, but there's an issue with zson Marshaling nil
	// interfaces, which occurs frequently with zjsonio.Object.Types. For now
	// just reflect here.
	return w.encoder.Encode(describe{
		Type:  reflect.TypeOf(v).Name(),
		Value: v,
	})
}

func (w *ZJSONWriter) Close() error {
	return nil
}
