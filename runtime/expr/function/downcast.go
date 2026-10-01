package function

import (
	"github.com/superdb/super"
	"github.com/superdb/super/runtime/expr"
	"github.com/superdb/super/vector"
)

type downcast struct {
	downcast *expr.Downcast
}

func (d *downcast) ApplyOpt() vector.ApplyOpt { return vector.ApplyNone }

func newDowncast(sctx *super.Context) *downcast {
	return &downcast{downcast: expr.NewDowncast(sctx)}
}

func (d *downcast) Call(vecs ...vector.Any) vector.Any {
	return d.downcast.To(vecs[0], vecs[1])
}
