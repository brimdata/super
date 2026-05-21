package vcache

import (
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
)

type bytes struct {
	mu    sync.Mutex
	meta  *csup.Bytes
	len   uint32
	table *vector.BytesTable
}

func newBytes(cctx *csup.Context, meta *csup.Bytes) *bytes {
	return &bytes{meta: meta, len: meta.Len(cctx)}
}

func (b *bytes) length() uint32 {
	return b.len
}

func (*bytes) unmarshal(*csup.Context, field.Projection) {}

func (b *bytes) project(loader *loader, projection field.Projection) vector.Any {
	if len(projection) > 0 {
		return vector.NewMissing(loader.sctx, b.length())
	}
	table := b.load(loader)
	switch b.meta.Typ.ID() {
	case super.IDString:
		return vector.NewString(table)
	case super.IDBytes:
		return vector.NewBytes(table)
	default:
		panic(b.meta.Typ)
	}
}

func (b *bytes) load(loader *loader) vector.BytesTable {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.table != nil {
		return *b.table
	}
	table := loadBytesTable(loader, b.meta.Offsets, b.meta.Bytes)
	b.table = &table
	return table
}

func loadBytesTable(loader *loader, offsets, bytes csup.Segment) vector.BytesTable {
	offs, err := csup.ReadUint32s(offsets, loader.r)
	if err != nil {
		panic(err)
	}
	if len(offs) == 0 {
		offs = []uint32{0}
	}
	b := make([]byte, bytes.MemLength)
	if err := bytes.Read(loader.r, b); err != nil {
		panic(err)
	}
	return vector.NewBytesTable(offs, b)
}
