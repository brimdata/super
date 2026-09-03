package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
)

type Quiet struct {
	defuse *expr.Defuse
}

func newQuiet(sctx *super.Context) *Quiet {
	return &Quiet{expr.NewDefuse(sctx)}
}

func (q *Quiet) Call(args ...vector.Any) vector.Any {
	return vector.Apply(vector.ApplyNone, q.call, q.defuse.Eval(args[0]))
}

func (q *Quiet) call(args ...vector.Any) vector.Any {
	vec := args[0]
	if k := vec.Kind(); k == vector.KindError {
		return vector.NewNone(vec.Len())
	}
	return vec
}

func (q *Quiet) ApplyOpt() vector.ApplyOpt { return vector.ApplyNone }
