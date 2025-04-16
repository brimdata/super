package vcache

import (
	"sync"

	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type float struct {
	mu   sync.Mutex
	meta *csup.Float
	count
	vals  []float64
	nulls *nulls
}

func newFloat(cctx *csup.Context, meta *csup.Float, nulls *nulls) *float {
	return &float{
		meta:  meta,
		nulls: nulls,
		count: count{meta.Len(cctx), nulls.count()},
	}
}

func (*float) unmarshal(*csup.Context, field.Projection) {}

func (i *float) project(loader *loader, projection field.Projection) vector.Any {
	if len(projection) > 0 {
		return vector.NewMissing(loader.sctx, i.length())
	}
	vals, nulls := i.load(loader)
	return vector.NewFloat(i.meta.Typ, vals, nulls)
}

func (i *float) load(loader *loader) ([]float64, bitvec.Bits) {
	nulls := i.nulls.get(loader)
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.vals != nil {
		return i.vals, nulls
	}
	bytes := make([]byte, i.meta.Location.MemLength)
	if err := i.meta.Location.Read(loader.r, bytes); err != nil {
		panic(err)
	}
	vals := byteconv.ReinterpretSlice[float64](bytes)
	i.vals = extendForNulls(vals, nulls, i.count)
	return i.vals, nulls
}

type floatLoader struct {
	loader *loader
	shadow *float
}

var _ vector.FloatLoader = (*floatLoader)(nil)

func (i *floatLoader) Load() ([]float64, bitvec.Bits) {
	return i.shadow.load(i.loader)
}
