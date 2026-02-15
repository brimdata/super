package vcache

import (
	"sync"

	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
)

type error_ struct {
	mu     sync.Mutex
	meta   *csup.Error
	len    uint32
	values shadow
}

func newError(cctx *csup.Context, meta *csup.Error) *error_ {
	return &error_{meta: meta, len: meta.Len(cctx)}
}

func (e *error_) length() uint32 {
	return e.len
}

func (e *error_) unmarshal(cctx *csup.Context, projection field.Projection) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.values == nil {
		e.values = newShadow(cctx, e.meta.Values)
	}
	e.values.unmarshal(cctx, projection)
}

func (e *error_) project(loader *loader, projection field.Projection) vector.Any {
	vec := e.values.project(loader, projection)
	typ := loader.sctx.LookupTypeError(vec.Type())
	return vector.NewError(typ, vec)
}
