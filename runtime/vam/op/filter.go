package op

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
)

type Filter struct {
	zctx   *super.Context
	parent vector.Puller
	expr   expr.Evaluator
}

func NewFilter(zctx *super.Context, parent vector.Puller, expr expr.Evaluator) *Filter {
	return &Filter{zctx, parent, expr}
}

func (f *Filter) Pull(done bool) (vector.Any, error) {
	for {
		vec, err := f.parent.Pull(done)
		if vec == nil || err != nil {
			return nil, err
		}
		if masked, ok := applyMask(vec, f.expr.Eval(vec)); ok {
			return masked, nil
		}
	}
}

// applyMask applies the mask vector mask to vec.  Elements of mask that are not
// Boolean are considered false.
func applyMask(vec, mask vector.Any) (vector.Any, bool) {
	n := mask.Len()
	var index []uint32
	for k := uint32(0); k < n; k++ {
		if vector.BoolValue(mask, k) {
			index = append(index, k)
		}
	}
	if len(index) == 0 {
		return nil, false
	}
	if len(index) == int(n) {
		return vec, true
	}
	return vector.NewView(index, vec), true
}
