package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
)

type downcast struct {
	downcast *expr.Downcast
	defuser  *expr.Defuse
}

func (d *downcast) ApplyOpt() vector.ApplyOpt { return vector.ApplyNone }

func newDowncast(sctx *super.Context) *downcast {
	return &downcast{
		downcast: expr.NewDowncast(sctx),
		defuser:  expr.NewDefuse(sctx),
	}
}

func (d *downcast) Call(vecs ...vector.Any) vector.Any {
	return vector.Apply(vector.ApplyNone, func(vecs ...vector.Any) vector.Any {
		return d.downcast.To(vecs[0], vecs[1])
	}, d.defuser.Eval(vecs[0]), d.defuser.Eval(vecs[1]))
}
