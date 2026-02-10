package expr

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/vector"
)

// Renamer renames one or more fields in a record.  See [expr.Renamer], on which
// it relies, for more detail.
type Renamer struct {
	sctx    *super.Context
	renamer *expr.Renamer
}

func NewRenamer(sctx *super.Context, srcs, dsts []*expr.Lval) *Renamer {
	return &Renamer{sctx, expr.NewRenamer(sctx, srcs, dsts)}
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
	val, err := r.renamer.EvalToValAndError(super.NewValue(vec.Type(), nil))
	if err != nil {
		return vector.NewWrappedError(r.sctx, err.Error(), vec)
	}
	return changeRecordType(recVec, val.Type().(*super.TypeRecord))
}

func changeRecordType(vec *vector.Record, typ *super.TypeRecord) *vector.Record {
	fields := slices.Clone(vec.Fields)
	for i, f := range typ.Fields {
		if rtyp, ok := f.Type.(*super.TypeRecord); ok {
			fields[i] = changeRecordType(vec.Fields[i].(*vector.Record), rtyp)
		}
	}
	return vector.NewRecord(typ, fields, vec.Len(), vec.Nulls)
}
