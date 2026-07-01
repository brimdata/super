package csup

import (
	"bytes"
	"fmt"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr/agg"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/bsupio"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/vbuild"
	"github.com/brimdata/super/vector/vio"
)

var maxObjectSize uint32 = 120_000

// Serializer implements the vio.Pusher interface. A Pusher creates a vector
// CSUP object from a stream of vector.Any.
type Serializer struct {
	writer    io.WriteCloser
	dynamic   *vbuild.DynamicBuilder
	fuser     *agg.Fuser
	fuserSctx *super.Context
	size      uint64
}

var _ vio.Pusher = (*Serializer)(nil)

func NewSerializer(w io.WriteCloser) *Serializer {
	return &Serializer{
		writer:  w,
		dynamic: vbuild.NewDynamicBuilder(),
	}
}

func (w *Serializer) Close() error {
	firstErr := w.finalizeObject()
	if firstErr == nil {
		firstErr = w.writeFooter()
	}
	if err := w.writer.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (w *Serializer) Push(vec vector.Any) error {
	if vec.Len() != 0 {
		w.dynamic.Write(vec)
		if w.dynamic.Len() >= maxObjectSize {
			return w.finalizeObject()
		}
	}
	return nil
}

func (w *Serializer) finalizeObject() error {
	vec := w.dynamic.BuildDynamic()
	if vec.Len() == 0 {
		return nil
	}
	w.fuse(vec)
	enc := NewDynamicEncoder(vec)
	root, dataSize, err := enc.Encode()
	if err != nil {
		return fmt.Errorf("system error: could not encode CSUP metadata: %w", err)
	}
	// At this point all the vector data has been written out
	// to the underlying spiller, so we start writing BSUP at this point.
	var metaBuf bytes.Buffer
	zw := bsupio.NewWriter(sio.NopCloser(&metaBuf))
	// First, we write the root segmap of the vector of integer type IDs.
	cctx := enc.cctx
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
	if err := zw.Write(buildTypeDefsValue(cctx)); err != nil {
		return fmt.Errorf("could not write CSUP metadata: %w", err)
	}
	zw.EndStream()
	typeSize := zw.Position() - metaSize
	// Header
	if _, err := w.writer.Write(Header{Version, SectionObject}.Serialize()); err != nil {
		return fmt.Errorf("system error: could not write CSUP header: %w", err)
	}
	// DataHeader
	o := DataHeader{uint64(metaSize), uint64(typeSize), dataSize, uint32(root)}
	if _, err := w.writer.Write(o.Serialize()); err != nil {
		return fmt.Errorf("system error: could not write CSUP header: %w", err)
	}
	w.size += HeaderSize + o.Size()
	// Metadata section
	if _, err := w.writer.Write(metaBuf.Bytes()); err != nil {
		return fmt.Errorf("system error: could not write CSUP metadata section: %w", err)
	}
	// Data section
	if err := enc.Emit(w.writer); err != nil {
		return fmt.Errorf("system error: could not write CSUP data section: %w", err)
	}
	// Set new dynamic so we can write the next object.
	w.dynamic = vbuild.NewDynamicBuilder()
	return nil
}

func (w *Serializer) fuse(dynamic *vector.Dynamic) {
	if w.fuser == nil {
		w.fuserSctx = super.NewContext()
		w.fuser = agg.NewFuser(w.fuserSctx, false)
	}
	for _, vec := range dynamic.Values {
		typ, err := w.fuserSctx.TranslateType(vec.Type())
		if err != nil {
			panic(err)
		}
		w.fuser.Fuse(typ)
	}
}

func (w *Serializer) writeFooter() error {
	if w.fuser == nil {
		return nil
	}
	fusedBytes := super.EncodeTypeValue(w.fuser.Type())
	if _, err := w.writer.Write(Header{Version, SectionFooter}.Serialize()); err != nil {
		return err
	}
	f := Footer{uint32(len(fusedBytes))}
	if _, err := w.writer.Write(f.Serialize()); err != nil {
		return err
	}
	if _, err := w.writer.Write(fusedBytes); err != nil {
		return err
	}
	w.size += HeaderSize + f.Size()
	if _, err := w.writer.Write(Trailer{w.size, uint32(len(fusedBytes))}.Serialize()); err != nil {
		return err
	}
	return nil
}

func buildTypeDefsValue(cctx *Context) super.Value {
	var bytes []byte
	if cctx.typedefs != nil {
		bytes = cctx.typedefs.Bytes()
	}
	return super.NewBytes(super.EncodeBytes(bytes))
}

// XXX ValWriter provides a temporary interface to support writing super.Values
// to CSUP.  We should remove this at some point in factor of vector-only writes.
type ValWriter struct {
	sctx       *super.Context
	serializer *Serializer
	builder    *vector.DynamicValueBuilder
}

var _ sio.Writer = (*ValWriter)(nil)

func NewValWriter(w io.WriteCloser) *ValWriter {
	sctx := super.NewContext()
	return &ValWriter{
		sctx:       sctx,
		serializer: NewSerializer(w),
		builder:    vector.NewDynamicValueBuilder(),
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
