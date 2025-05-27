package csup

import (
	"io"
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
	"github.com/brimdata/super/zio/bsupio"
	"golang.org/x/sync/errgroup"
)

const bsupMaxValues = 256

type bsupEncoder struct {
	typ    super.Type
	writer *bsupio.Writer

	bytes   []zcode.Bytes
	encoder Encoder
}

// newBSUPEncoder returns an encoder with behavior dependent on the number of
// values written to it.  If more than bsupMaxValues are written, they are
// encoded with an encoder obtained from NewEncoder.  Otherwise, they are
// encoded as BSUP by writing them to w.
func newBSUPEncoder(t super.Type, w *bsupio.Writer) *bsupEncoder {
	return &bsupEncoder{typ: t, writer: w}
}

func (b *bsupEncoder) Write(bytes zcode.Bytes) {
	if b.encoder != nil {
		b.encoder.Write(bytes)
		return
	}
	b.bytes = append(b.bytes, slices.Clone(bytes))
	if len(b.bytes) > bsupMaxValues {
		b.encoder = NewEncoder(b.typ)
		for _, bb := range b.bytes {
			b.encoder.Write(bb)
		}
		b.bytes = nil
	}
}

func (b *bsupEncoder) Encode(g *errgroup.Group) {
	if b.encoder != nil {
		b.encoder.Encode(g)
		return
	}
	for _, bb := range b.bytes {
		val := super.NewValue(b.typ, bb)
		if err := b.writer.Write(val); err != nil {
			g.Go(func() error { return err })
			return
		}
	}
}

func (b *bsupEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	if b.encoder != nil {
		return b.encoder.Metadata(cctx, off)
	}
	return off, cctx.enter(&BSUP{uint32(len(b.bytes))})
}

func (b *bsupEncoder) Emit(w io.Writer) error {
	if b.encoder != nil {
		return b.encoder.Emit(w)
	}
	// Nothing to do since Encode wrote everything to b.writer.
	return nil
}
