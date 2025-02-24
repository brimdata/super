package expr

import (
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

//go:generate go run gendatepart.go

type datePartExpr struct {
	zctx *super.Context
	expr Evaluator
	fn   func(vector.Any) vector.Any
}

func NewDatePartExpr(zctx *super.Context, part string, e Evaluator) (Evaluator, error) {
	fn, ok := datePartFuncs[part]
	if !ok {
		return nil, fmt.Errorf("date_part: %q not supported", part)
	}
	return &datePartExpr{zctx, e, fn}, nil
}

func (d *datePartExpr) Eval(this vector.Any) vector.Any {
	return vector.Apply(true, d.eval, d.expr.Eval(this))
}

func (d *datePartExpr) eval(vecs ...vector.Any) vector.Any {
	vec := vector.Under(vecs[0])
	if vec.Type().ID() != super.IDTime {
		return vector.NewWrappedError(d.zctx, "date_part: time value required", vecs[0])
	}
	return d.fn(vec)
}
