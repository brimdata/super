package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

// Putter adapts the behavior of recordExpr (obtained from NewRecordExpr) to
// match that of the put operator, which emits an error when an input value is
// not a record.
type Putter struct {
	zctx       *super.Context
	recordExpr Evaluator
}

func NewPutter(zctx *super.Context, recordExpr Evaluator) *Putter {
	return &Putter{zctx, recordExpr}
}

func (p *Putter) Eval(vec vector.Any) vector.Any {
	return vector.Apply(false, p.eval, vec)
}

func (p *Putter) eval(vecs ...vector.Any) vector.Any {
	vec := vecs[0]
	if vec.Type().Kind() != super.RecordKind {
		return vector.NewWrappedError(p.zctx, "put: not a record", vec)
	}
	return p.recordExpr.Eval(vec)
}
