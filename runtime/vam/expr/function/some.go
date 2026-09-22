package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
)

type Some struct {
	sctx   *super.Context
	defuse *expr.Defuse
}

func newSome(sctx *super.Context) *Some {
	return &Some{sctx, expr.NewDefuse(sctx)}
}

func (s *Some) Call(args ...vector.Any) vector.Any {
	return vector.Apply(vector.ApplyNone, s.call, s.defuse.Eval(args[0]))
}

func (s *Some) call(args ...vector.Any) vector.Any {
	vec := args[0]
	switch vec.Kind() {
	case vector.KindError, vector.KindOption, vector.KindNone:
		return vec
	default:
		return vector.NewOption(s.sctx.LookupTypeOption(vec.Type()), vec)
	}
}

func (*Some) ApplyOpt() vector.ApplyOpt { return vector.ApplyNone }
