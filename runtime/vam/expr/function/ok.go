package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
)

type Ok struct {
	defuse *expr.Defuse
}

func newOk(sctx *super.Context) *Ok {
	return &Ok{expr.NewDefuse(sctx)}
}

func (o *Ok) Call(args ...vector.Any) vector.Any {
	return vector.Apply(vector.ApplyNone, o.call, o.defuse.Eval(args[0]))
}

func (o *Ok) call(args ...vector.Any) vector.Any {
	vec := args[0]
	if vec.Kind() == vector.KindError {
		return vector.NewNone(vec.Len())
	}
	return vec
}

func (*Ok) ApplyOpt() vector.ApplyOpt { return vector.ApplyNone }
