package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr/coerce"
	"github.com/brimdata/super/runtime/vam/expr/cast"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

// coerceVals checks if a and b are type compatible for comparison
// and/or math and modifies one of the vectors promoting to the
// other's type as needed according to Zed's coercion rules (modified
// vectors are returned, not changed in place).  When errors are
// encountered an error vector is returned and the coerced values
// are abandoned.
func coerceVals(sctx *super.Context, a, b vector.Any) (vector.Any, vector.Any, vector.Any) {
	aid := a.Type().ID()
	bid := b.Type().ID()
	if aid == bid {
		//XXX this catches complex types so we need to add support
		// for things like {a:10}<{a:123} or [1,2]+[3,4]
		// sam doesn't support this yet.
		return a, b, nil
	}
	if aid == super.IDNull {
		a = vector.NewConst(super.NewValue(b.Type(), nil), b.Len(), bitvec.Zero)
		return a, b, nil //XXX
	}
	if bid == super.IDNull {
		b = vector.NewConst(super.NewValue(a.Type(), nil), a.Len(), bitvec.Zero)
		return a, b, nil //XXX
	}
	if !super.IsNumber(aid) || !super.IsNumber(bid) {
		return nil, nil, vector.NewStringError(sctx, coerce.ErrIncompatibleTypes.Error(), a.Len())
	}
	id, err := coerce.Promote(super.NewValue(a.Type(), nil), super.NewValue(b.Type(), nil))
	if err != nil {
		panic(err)
	}
	typ, err := super.LookupPrimitiveByID(id)
	if err != nil {
		panic(err)
	}
	return cast.To(sctx, a, typ), cast.To(sctx, b, typ), nil
}
