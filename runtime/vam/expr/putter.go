package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type putter struct {
	sctx *super.Context
	e    Evaluator
}

// NewPutter wraps e to implement the behavior of the put operator, which emits
// an error when an input value is not a record.
func NewPutter(sctx *super.Context, e Evaluator) Evaluator {
	return &putter{sctx, e}
}

func (p *putter) Eval(vec vector.Any) vector.Any {
	return vector.Apply(false, p.eval, vec)
}

func (p *putter) eval(vecs ...vector.Any) vector.Any {
	vec := vecs[0]
	if k := vec.Type().Kind(); k != super.RecordKind {
		if k == super.ErrorKind {
			return vec
		}
		return vector.NewWrappedError(p.sctx, "put: not a record", vec)
	}
	return p.e.Eval(vec)
}
