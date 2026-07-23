package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
)

type defuse struct {
	defuse expr.Evaluator
}

func newDefuse(sctx *super.Context) *defuse {
	return &defuse{defuse: expr.NewDefuse(sctx)}
}

func (*defuse) ApplyOpt() vector.ApplyOpt { return vector.ApplyNone }

func (d *defuse) Call(args ...vector.Any) vector.Any {
	return d.defuse.Eval(args[0])
}
