package vcache

import (
	"sync"

	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
)

type fusion struct {
	mu     sync.Mutex
	meta   *csup.Fusion
	len    uint32 //XXX
	values shadow
	//XXX currently stored as typevals... need to change this to a typeID based encoding
	// so that the types are efficiently interned
	subTypes shadow
}

func newFusion(cctx *csup.Context, meta *csup.Fusion) *fusion {
	return &fusion{meta: meta, len: meta.Len(cctx)}
}

func (f *fusion) length() uint32 {
	return f.len
}

func (f *fusion) unmarshal(cctx *csup.Context, projection field.Projection) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.values == nil {
		f.values = newShadow(cctx, f.meta.Values)
		//XXX for now just read in all the typevals
		f.subTypes = newShadow(cctx, f.meta.SubTypes)
	}
	f.values.unmarshal(cctx, projection)
	f.subTypes.unmarshal(cctx, projection)
}

func (f *fusion) project(loader *loader, projection field.Projection) vector.Any {
	vec := f.values.project(loader, projection)
	subTypes := f.subTypes.project(loader, nil)
	typ := loader.sctx.LookupTypeFusion(vec.Type())
	return vector.NewFusion(typ, vec, subTypes.(*vector.TypeValue))
}
