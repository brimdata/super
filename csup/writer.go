package csup

import (
	"bytes"
	"fmt"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/bsupio"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/vio"
)

var maxObjectSize uint32 = 120_000

// Serializer implements the vio.Pusher interface. A Pusher creates a vector
// CSUP object from a stream of vector.Any.
type Serializer struct {
	writer  io.WriteCloser
	dynamic *DynamicEncoder
}

var _ vio.Pusher = (*Serializer)(nil)

func NewSerializer(w io.WriteCloser) *Serializer {
	return &Serializer{
		writer:  w,
		dynamic: NewDynamicEncoder(),
	}
}

func (w *Serializer) Close() error {
	firstErr := w.finalizeObject()
	if err := w.writer.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (w *Serializer) Push(vec vector.Any) error {
	if vec.Len() != 0 {
		w.dynamic.Write(vec)
		if w.dynamic.len >= maxObjectSize {
			return w.finalizeObject()
		}
	}
	return nil
}

func (w *Serializer) finalizeObject() error {
	root, dataSize, err := w.dynamic.Encode()
	if err != nil {
		return fmt.Errorf("system error: could not encode CSUP metadata: %w", err)
	}
	// At this point all the vector data has been written out
	// to the underlying spiller, so we start writing BSUP at this point.
	var metaBuf bytes.Buffer
	zw := bsupio.NewWriter(sio.NopCloser(&metaBuf))
	// First, we write the root segmap of the vector of integer type IDs.
	cctx := w.dynamic.cctx
	m := sup.NewBSUPMarshalerWithContext(cctx.local)
	m.Decorate(sup.StyleSimple)
	for id := range len(cctx.metas) {
		val, err := m.Marshal(cctx.Lookup(ID(id)))
		if err != nil {
			return fmt.Errorf("could not marshal CSUP metadata: %w", err)
		}
		if err := zw.Write(val); err != nil {
			return fmt.Errorf("could not write CSUP metadata: %w", err)
		}
	}
	zw.EndStream()
	metaSize := zw.Position()
	// Header
	if _, err := w.writer.Write(Header{Version, uint64(metaSize), dataSize, uint32(root)}.Serialize()); err != nil {
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
	// Set new dynamic so we can write the next object.
	w.dynamic = NewDynamicEncoder()
	return nil
}

// XXX ValWriter provides a temporary interface to support writing super.Values
// to CSUP.  We should remove this at some point in factor of vector-only writes.
type ValWriter struct {
	sctx       *super.Context
	serializer *Serializer
	builder    *vector.DynamicBuilder
}

var _ sio.Writer = (*ValWriter)(nil)

func NewValWriter(w io.WriteCloser) *ValWriter {
	return &ValWriter{
		sctx:       super.NewContext(), //XXX
		serializer: NewSerializer(w),
		builder:    vector.NewDynamicBuilder(),
	}
}

func (v *ValWriter) Write(val super.Value) error {
	v.builder.Write(val)
	return nil
}

func (v *ValWriter) Close() error {
	err := v.serializer.Push(v.builder.Build(v.sctx))
	if closeErr := v.serializer.Close(); err == nil {
		err = closeErr
	}
	return err
}
