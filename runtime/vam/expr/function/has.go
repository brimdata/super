package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
)

type Has struct {
	missing Missing
	not     *expr.Not
}

func newHas(sctx *super.Context) *Has {
	return &Has{missing: Missing{sctx}, not: expr.NewLogicalNot(sctx, &expr.This{})}
}

func (h *Has) Call(args ...vector.Any) vector.Any {
	return h.not.Eval(h.missing.Call(args...))
}

type Missing struct {
	sctx *super.Context
}

func (*Missing) NoDefuse() bool            { return true }
func (*Missing) ApplyOpt() vector.ApplyOpt { return vector.ApplyRipFusions | vector.ApplyRipUnions }

func (m *Missing) Call(args ...vector.Any) vector.Any {
	n := args[0].Len()
	for _, vec := range args {
		vec = vector.DeoptionWithNone(m.sctx, vec)
		if _, ok := vec.(*vector.None); ok {
			return vector.NewConstBool(true, n)
		}
	}
	return vector.NewConstBool(false, n)
}
