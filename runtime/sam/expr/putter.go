package expr

import (
	"github.com/brimdata/super"
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

func (p *putter) Eval(val super.Value) super.Value {
	if k := val.Type().Kind(); k != super.RecordKind {
		if k == super.ErrorKind {
			return val
		}
		return p.sctx.WrapError("put: not a record", val)
	}
	return p.e.Eval(val)
}
