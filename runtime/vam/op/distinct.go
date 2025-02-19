package op

import (
	"encoding/binary"

	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zcode"
)

type Distinct struct {
	parent vector.Puller
	expr   expr.Evaluator

	blocked map[string]struct{}
	key     []byte
}

func NewDistinct(parent vector.Puller, expr expr.Evaluator) *Distinct {
	return &Distinct{parent, expr, map[string]struct{}{}, nil}
}

func (d *Distinct) Pull(done bool) (vector.Any, error) {
	for {
		vec, err := d.parent.Pull(done)
		if vec == nil || err != nil {
			clear(d.blocked)
			return nil, err
		}
		var b zcode.Builder
		var index []uint32
		keyVec := d.expr.Eval(vec)
		for i := range keyVec.Len() {
			b.Truncate()
			keyVal := vectorValue(&b, keyVec, i)
			d.key = binary.LittleEndian.AppendUint32(d.key[:0], uint32(keyVal.Type().ID()))
			d.key = append(d.key, keyVal.Bytes()...)
			if _, ok := d.blocked[string(d.key)]; !ok {
				d.blocked[string(d.key)] = struct{}{}
				index = append(index, i)
			}
		}
		if len(index) > 0 {
			return vector.NewView(vec, index), nil
		}
	}
}
