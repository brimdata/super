package function

import (
	"github.com/brimdata/super"
	samexpr "github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/runtime/sam/expr/function"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
)

type caster struct {
	caster *samFunc
	defuse *expr.Defuse
}

func newCaster(sctx *super.Context) expr.Function {
	fn := newSamFunc(sctx, function.NewCaster(sctx).(samexpr.Function))
	return &caster{fn, expr.NewDefuse(sctx)}
}

func (c *caster) ApplyOpts() vector.ApplyOpt { return vector.ApplyRipUnions }

func (c *caster) Call(vecs ...vector.Any) vector.Any {
	return c.caster.Call(c.defuse.Eval(vecs[0]), c.defuse.Eval(vecs[1]))
}
