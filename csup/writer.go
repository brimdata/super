package csup

import (
	"bytes"
	"fmt"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/bsupio"
)

var maxObjectSize uint32 = 120_000

// Writer implements the zio.Writer interface. A Writer creates a vector
// CSUP object from a stream of super.Records.
type Writer struct {
	zctx    *super.Context
	writer  io.WriteCloser
	dynamic *DynamicEncoder
}

var _ zio.Writer = (*Writer)(nil)

func NewWriter(w io.WriteCloser) *Writer {
	return &Writer{
		zctx:    super.NewContext(),
		writer:  w,
		dynamic: NewDynamicEncoder(),
	}
}

func (w *Writer) Close() error {
	firstErr := w.finalizeObject()
	if err := w.writer.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (w *Writer) Write(val super.Value) error {
	if err := w.dynamic.Write(val); err != nil {
		return err
	}
	if w.dynamic.len >= maxObjectSize {
		return w.finalizeObject()
	}
	return nil
}

func (w *Writer) finalizeObject() error {
	meta, dataSize, err := w.dynamic.Encode()
	if err != nil {
		return fmt.Errorf("system error: could not encode CSUP metadata: %w", err)
	}
	// At this point all the vector data has been written out
	// to the underlying spiller, so we start writing BSUP at this point.
	var metaBuf bytes.Buffer
	zw := bsupio.NewWriter(zio.NopCloser(&metaBuf))
	// First, we write the root segmap of the vector of integer type IDs.
	m := sup.NewBSUPMarshalerWithContext(w.zctx)
	m.Decorate(sup.StyleSimple)
	val, err := m.Marshal(meta)
	if err != nil {
		return fmt.Errorf("system error: could not marshal CSUP metadata: %w", err)
	}
	if err := zw.Write(val); err != nil {
		return fmt.Errorf("system error: could not serialize CSUP metadata: %w", err)
	}
	zw.EndStream()
	metaSize := zw.Position()
	// Header
	if _, err := w.writer.Write(Header{Version, uint64(metaSize), dataSize}.Serialize()); err != nil {
		return fmt.Errorf("system error: could not write CSUP header: %w", err)
	}
	// Metadata section
	if _, err := w.writer.Write(metaBuf.Bytes()); err != nil {
		return fmt.Errorf("system error: could not write CSUP metadata section: %w", err)
	}
	// Data section
	if err := w.dynamic.Emit(w.writer); err != nil {
		return fmt.Errorf("system error: could not write CSUP data section: %w", err)
	}
	// Set new dynamic so we can write the next section.
	w.dynamic = NewDynamicEncoder()
	w.zctx.Reset()
	return nil
}
