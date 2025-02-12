package distinct

import (
	"encoding/binary"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/zbuf"
)

type Op struct {
	parent zbuf.Puller
	expr   expr.Evaluator
	cache  []byte
	block  map[string]struct{}
}

func New(parent zbuf.Puller, expr expr.Evaluator) *Op {
	return &Op{
		parent: parent,
		expr:   expr,
		cache:  make([]byte, 0, 1024),
	}
}

func (o *Op) Pull(done bool) (zbuf.Batch, error) {
	if o.block == nil {
		o.block = make(map[string]struct{})
	}
	for {
		batch, err := o.parent.Pull(done)
		if batch == nil || err != nil {
			clear(o.block)
			return nil, err
		}
		vals := batch.Values()
		out := make([]super.Value, 0, len(vals))
		bytes := o.cache[:0]
		for i := range vals {
			val := o.expr.Eval(batch, vals[i])
			binary.LittleEndian.PutUint32(bytes[:4], uint32(val.Type().ID()))
			bytes = append(bytes[:4], val.Bytes()...)
			if _, ok := o.block[string(bytes)]; !ok {
				o.block[string(bytes)] = struct{}{}
				out = append(out, vals[i])
			}
		}
		if len(out) > 0 {
			o.cache = bytes
			return zbuf.NewBatch(batch, out), nil
		}
		batch.Unref()
	}
}
