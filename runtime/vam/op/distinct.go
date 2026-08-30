package op

import (
	"encoding/binary"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/vio"
)

type Distinct struct {
	sctx   *super.Context
	parent vio.Puller
	expr   expr.Evaluator

	blocked map[string]struct{}
	key     []byte
}

func NewDistinct(sctx *super.Context, parent vio.Puller, expr expr.Evaluator) *Distinct {
	return &Distinct{sctx, parent, expr, map[string]struct{}{}, nil}
}

func (d *Distinct) Pull(done bool) (vector.Any, error) {
	for {
		vec, err := d.parent.Pull(done)
		if vec == nil || err != nil {
			clear(d.blocked)
			return nil, err
		}
		var sb scode.Builder
		var index []uint32
		keyVec := d.expr.Eval(vec)
		// XXX In a future PR we will propagate nones as structured errors encountered here; they shouldn't be silently hidden
		//XXX see comment above
		keyVec = vector.DeoptionWithNone(d.sctx, keyVec)
		for i := range keyVec.Len() {
			keyVal := vector.ValueAt(&sb, keyVec, i)
			d.key = binary.LittleEndian.AppendUint32(d.key[:0], uint32(keyVal.Type().ID()))
			d.key = append(d.key, keyVal.Bytes()...)
			if _, ok := d.blocked[string(d.key)]; !ok {
				d.blocked[string(d.key)] = struct{}{}
				index = append(index, i)
			}
		}
		if len(index) > 0 {
			return vector.Pick(vec, index), nil
		}
	}
}
