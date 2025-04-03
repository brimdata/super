package seekindex

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zio"
)

type Writer struct {
	marshal *sup.MarshalZNGContext
	writer  zio.WriteCloser
	offset  uint64
	valoff  uint64
}

func NewWriter(w zio.WriteCloser) *Writer {
	return &Writer{
		marshal: sup.NewZNGMarshaler(),
		writer:  w,
	}
}

func (w *Writer) Write(min, max super.Value, valoff uint64, offset uint64) error {
	val, err := w.marshal.Marshal(&Entry{
		Min:    min,
		Max:    max,
		ValOff: w.valoff,
		ValCnt: valoff - w.valoff,
		Offset: w.offset,
		Length: offset - w.offset,
	})
	w.valoff = valoff
	w.offset = offset
	if err != nil {
		return err
	}
	return w.writer.Write(val)
}

func (w *Writer) Close() error { return w.writer.Close() }
