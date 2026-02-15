package vcache

import (
	"sync"

	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
)

type float struct {
	mu   sync.Mutex
	meta *csup.Float
	len  uint32
	vals []float64
}

func newFloat(cctx *csup.Context, meta *csup.Float) *float {
	return &float{meta: meta, len: meta.Len(cctx)}
}

func (f *float) length() uint32 {
	return f.len
}

func (*float) unmarshal(*csup.Context, field.Projection) {}

func (f *float) project(loader *loader, projection field.Projection) vector.Any {
	if len(projection) > 0 {
		return vector.NewMissing(loader.sctx, f.length())
	}
	return vector.NewFloat(f.meta.Typ, f.load(loader))
}

func (f *float) load(loader *loader) []float64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.vals != nil {
		return f.vals
	}
	bytes := make([]byte, f.meta.Location.MemLength)
	if err := f.meta.Location.Read(loader.r, bytes); err != nil {
		panic(err)
	}
	f.vals = byteconv.ReinterpretSlice[float64](bytes)
	return f.vals
}
