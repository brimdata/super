package vcache

import (
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type bytes struct {
	mu   sync.Mutex
	meta *csup.Bytes
	count
	nulls *nulls
	table *vector.BytesTable
}

func newBytes(cctx *csup.Context, meta *csup.Bytes, nulls *nulls) *bytes {
	return &bytes{
		meta:  meta,
		count: count{meta.Count, nulls.count()},
		nulls: nulls,
	}
}

func (*bytes) unmarshal(*csup.Context, field.Projection) {}

func (b *bytes) project(loader *loader, projection field.Projection) vector.Any {
	if len(projection) > 0 {
		return vector.NewMissing(loader.sctx, b.length())
	}
	table, nulls := b.load(loader)
	switch b.meta.Typ.ID() {
	case super.IDString:
		return vector.NewString(table, nulls)
	case super.IDBytes:
		return vector.NewBytes(table, nulls)
	case super.IDType:
		return vector.NewTypeValue(table, nulls)
	default:
		panic(b.meta.Typ)
	}
}

func (b *bytes) load(loader *loader) (vector.BytesTable, bitvec.Bits) {
	nulls := b.nulls.get(loader)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.table != nil {
		return *b.table, nulls
	}
	offsets, err := loadOffsets(loader.r, b.meta.Offsets, b.count, nulls)
	if err != nil {
		panic(err)
	}
	bytes := make([]byte, b.meta.Bytes.MemLength)
	if err := b.meta.Bytes.Read(loader.r, bytes); err != nil {
		panic(err)
	}
	table := vector.NewBytesTable(offsets, bytes)
	b.table = &table
	return table, nulls
}
