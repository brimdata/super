package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/vector"
)

// Renamer renames one or more fields in a record.  See [expr.Renamer], on which
// it relies, for more detail.
type Renamer struct {
	zctx    *zed.Context
	renamer *expr.Renamer
}

func NewRenamer(zctx *zed.Context, srcs, dsts []*expr.Lval) *Renamer {
	return &Renamer{zctx, expr.NewRenamer(zctx, srcs, dsts)}
}

func (r *Renamer) Eval(vec vector.Any) vector.Any {
	return vector.Apply(false, r.eval, vec)
}

func (r *Renamer) eval(vecs ...vector.Any) vector.Any {
	vec := vecs[0]
	recVec, ok := vector.Under(vec).(*vector.Record)
	if !ok {
		return vec
	}
	val, err := r.renamer.EvalToValAndError(nil, zed.NewValue(vec.Type(), nil))
	if err != nil {
		return vector.NewWrappedError(r.zctx, err.Error(), vec)
	}
	return vector.NewRecord(val.Type().(*zed.TypeRecord), recVec.Fields, recVec.Len(), recVec.Nulls)
}
