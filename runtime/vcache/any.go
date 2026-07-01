package vcache

import (
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
)

type _any struct {
	mu       sync.Mutex
	cctx     *csup.Context
	meta     *csup.Any
	len      uint32
	values   shadow
	subtypes *typevalue
}

func newAny(cctx *csup.Context, meta *csup.Any) *_any {
	return &_any{
		cctx: cctx,
		meta: meta,
		len:  meta.Len(cctx),
	}
}

func (f *_any) length() uint32 {
	return f.len
}

func (f *_any) unmarshal(cctx *csup.Context, projection field.Projection) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.values == nil {
		f.values = newShadow(cctx, f.meta.Values)
	}
	if f.subtypes == nil {
		f.subtypes = newTypeValue(cctx, cctx.Lookup(f.meta.Subtypes).(*csup.TypeValue))
	}
	f.values.unmarshal(cctx, projection)
}

func (f *_any) project(loader *loader, projection field.Projection) vector.Any {
	vec := f.values.project(loader, projection)
	typ := loader.sctx.LookupTypeFusion(super.TypeAll)
	return vector.NewFusionWithLoader(loader.sctx, typ, f.subtypes.newLoader(loader), vec)
}
