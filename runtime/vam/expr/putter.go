package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

// Putter adapts the behavior of recordExpr (obtained from NewRecordExpr) to
// match that of the put operator, which emits an error when an input value is
// not a record.
type Putter struct {
	sctx       *super.Context
	recordExpr Evaluator
}

func NewPutter(sctx *super.Context, recordExpr Evaluator) *Putter {
	return &Putter{sctx, recordExpr}
}

func (p *Putter) Eval(vec vector.Any) vector.Any {
	return vector.Apply(false, p.eval, vec)
}

func (p *Putter) eval(vecs ...vector.Any) vector.Any {
	vec := vecs[0]
	if k := vec.Type().Kind(); k != super.RecordKind {
		if k == super.ErrorKind {
			return vec
		}
		return vector.NewWrappedError(p.sctx, "put: not a record", vec)
	}
	return p.recordExpr.Eval(vec)
}
