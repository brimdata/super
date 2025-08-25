package csup

import (
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"golang.org/x/sync/errgroup"
)

type MapEncoder struct {
	keys    Encoder
	values  Encoder
	offsets *offsetsEncoder
	count   uint32
}

func NewMapEncoder(typ *super.TypeMap) *MapEncoder {
	return &MapEncoder{
		keys:    NewEncoder(typ.KeyType),
		values:  NewEncoder(typ.ValType),
		offsets: newOffsetsEncoder(),
	}
}

func (m *MapEncoder) Write(body scode.Bytes) {
	m.count++
	var len uint32
	it := body.Iter()
	for !it.Done() {
		m.keys.Write(it.Next())
		m.values.Write(it.Next())
		len++
	}
	m.offsets.writeLen(len)
}

func (m *MapEncoder) Emit(w io.Writer) error {
	if err := m.offsets.Emit(w); err != nil {
		return err
	}
	if err := m.keys.Emit(w); err != nil {
		return err
	}
	return m.values.Emit(w)
}

func (m *MapEncoder) Metadata(cctx *Context, off uint64) (uint64, ID) {
	off, lens := m.offsets.Segment(off)
	off, keys := m.keys.Metadata(cctx, off)
	off, vals := m.values.Metadata(cctx, off)
	return off, cctx.enter(&Map{
		Lengths: lens,
		Keys:    keys,
		Values:  vals,
		Length:  m.count,
	})
}

func (m *MapEncoder) Encode(group *errgroup.Group) {
	m.offsets.Encode(group)
	m.keys.Encode(group)
	m.values.Encode(group)
}
