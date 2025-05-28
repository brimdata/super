package vcache

import (
	"sync"

	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type error_ struct {
	mu   sync.Mutex
	meta *csup.Error
	count
	values shadow
	nulls  *nulls
}

func newError(cctx *csup.Context, meta *csup.Error, nulls *nulls) *error_ {
	return &error_{
		meta:  meta,
		nulls: nulls,
		count: count{meta.Len(cctx), nulls.count()},
	}
}

func (e *error_) unmarshal(cctx *csup.Context, projection field.Projection) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.values == nil {
		e.values = newShadow(cctx, e.meta.Values, e.nulls)
	}
	e.values.unmarshal(cctx, projection)
}

func (e *error_) project(loader *loader, projection field.Projection) vector.Any {
	nulls := e.load(loader)
	vec := e.values.project(loader, projection)
	typ := loader.sctx.LookupTypeError(vec.Type())
	return vector.NewError(typ, vec, nulls)
}

func (e *error_) load(loader *loader) bitvec.Bits {
	return e.nulls.get(loader)
}
