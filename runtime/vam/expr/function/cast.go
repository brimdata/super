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
}

func (c *caster) ApplyOpts() vector.ApplyOpt { return vector.ApplyRipUnions }

func newCaster(sctx *super.Context) expr.Function {
	return &caster{newSamFunc(sctx, function.NewCaster(sctx).(samexpr.Function))}
}

func (c *caster) Call(vecs ...vector.Any) vector.Any {
	return c.caster.Call(vecs[0], vecs[1])
}
