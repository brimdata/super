package vcache

import (
	"sync"

	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type dict struct {
	mu   sync.Mutex
	meta *csup.Dict
	count
	nulls  *nulls
	values shadow
	counts []uint32 // number of each entry indexed by dict offset
	index  []byte   // dict offset of each value in vector
}

func newDict(cctx *csup.Context, meta *csup.Dict, nulls *nulls) *dict {
	return &dict{
		meta:  meta,
		nulls: nulls,
		count: count{meta.Len(cctx), nulls.count()},
	}
}

func (d *dict) unmarshal(cctx *csup.Context, projection field.Projection) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.values == nil {
		d.values = newShadow(cctx, d.meta.Values, nil)
	}
	d.values.unmarshal(cctx, projection)
}

func (d *dict) project(loader *loader, projection field.Projection) vector.Any {
	if len(projection) > 0 {
		return vector.NewMissing(loader.sctx, d.length())
	}
	index, counts, nulls := d.load(loader)
	return vector.NewDict(d.values.project(loader, projection), index, counts, nulls)
}

func (d *dict) load(loader *loader) ([]byte, []uint32, bitvec.Bits) {
	nulls := d.nulls.get(loader)
	d.mu.Lock()
	defer d.mu.Unlock()
	d.index = make([]byte, d.meta.Index.MemLength)
	if err := d.meta.Index.Read(loader.r, d.index); err != nil {
		panic(err)
	}
	d.index = extendForNulls(d.index, nulls, d.count)
	v, err := csup.ReadUint32s(d.meta.Counts, loader.r)
	if err != nil {
		panic(err)
	}
	d.counts = v
	return d.index, d.counts, nulls
}
