package vng

import (
	"bytes"
	"fmt"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/zngio"
	"github.com/brimdata/super/zson"
)

// Writer implements the zio.Writer interface. A Writer creates a vector
// VNG object from a stream of super.Records.
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
	firstErr := w.finalize()
	if err := w.writer.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (w *Writer) Write(val super.Value) error {
	return w.dynamic.Write(val)
}

func (w *Writer) finalize() error {
	meta, dataSize, err := w.dynamic.Encode()
	if err != nil {
		return fmt.Errorf("system error: could not encode VNG metadata: %w", err)
	}
	// At this point all the vector data has been written out
	// to the underlying spiller, so we start writing zng at this point.
	var metaBuf bytes.Buffer
	zw := zngio.NewWriter(zio.NopCloser(&metaBuf))
	// First, we write the root segmap of the vector of integer type IDs.
	m := zson.NewZNGMarshalerWithContext(w.zctx)
	m.Decorate(zson.StyleSimple)
	val, err := m.Marshal(meta)
	if err != nil {
		return fmt.Errorf("system error: could not marshal VNG metadata: %w", err)
	}
	if err := zw.Write(val); err != nil {
		return fmt.Errorf("system error: could not serialize VNG metadata: %w", err)
	}
	zw.EndStream()
	metaSize := zw.Position()
	// Header
	if _, err := w.writer.Write(Header{Version, uint64(metaSize), dataSize}.Serialize()); err != nil {
		return fmt.Errorf("system error: could not write VNG header: %w", err)
	}
	// Metadata section
	if _, err := w.writer.Write(metaBuf.Bytes()); err != nil {
		return fmt.Errorf("system error: could not write VNG metadata section: %w", err)
	}
	// Data section
	if err := w.dynamic.Emit(w.writer); err != nil {
		return fmt.Errorf("system error: could not write VNG data section: %w", err)
	}
	return nil
}
