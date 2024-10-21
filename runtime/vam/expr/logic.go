package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type Not struct {
	zctx *super.Context
	expr Evaluator
}

var _ Evaluator = (*Not)(nil)

func NewLogicalNot(zctx *super.Context, e Evaluator) *Not {
	return &Not{zctx, e}
}

func (n *Not) Eval(val vector.Any) vector.Any {
	val, ok := EvalBool(n.zctx, val, n.expr)
	if !ok {
		return val
	}
	b := val.(*vector.Bool)
	bits := make([]uint64, len(b.Bits))
	for k := range bits {
		bits[k] = ^b.Bits[k]
	}
	return b.CopyWithBits(bits)
}

type And struct {
	zctx *super.Context
	lhs  Evaluator
	rhs  Evaluator
}

func NewLogicalAnd(zctx *super.Context, lhs, rhs Evaluator) *And {
	return &And{zctx, lhs, rhs}
}

type Or struct {
	zctx *super.Context
	lhs  Evaluator
	rhs  Evaluator
}

func NewLogicalOr(zctx *super.Context, lhs, rhs Evaluator) *Or {
	return &Or{zctx, lhs, rhs}
}

func (a *And) Eval(val vector.Any) vector.Any {
	//XXX change this logic to handle dynamic instead of simple ok decision,
	// if there are any valid bools then we need to and them together
	lhs, ok := EvalBool(a.zctx, val, a.lhs)
	if !ok {
		//XXX mix errors
		return lhs
	}
	rhs, ok := EvalBool(a.zctx, val, a.rhs)
	if !ok {
		//XXX mix errors
		return rhs
	}
	blhs := lhs.(*vector.Bool)
	brhs := rhs.(*vector.Bool)
	if len(blhs.Bits) != len(brhs.Bits) {
		panic("length mistmatch")
	}
	bits := make([]uint64, len(blhs.Bits))
	for k := range bits {
		bits[k] = blhs.Bits[k] & brhs.Bits[k]
	}
	//XXX intersect nulls
	return blhs.CopyWithBits(bits)
}

func (o *Or) Eval(val vector.Any) vector.Any {
	lhs, ok := EvalBool(o.zctx, val, o.lhs)
	if !ok {
		return lhs
	}
	rhs, ok := EvalBool(o.zctx, val, o.rhs)
	if !ok {
		return rhs
	}
	blhs := lhs.(*vector.Bool)
	brhs := rhs.(*vector.Bool)
	bits := make([]uint64, len(blhs.Bits))
	if len(blhs.Bits) != len(brhs.Bits) {
		panic("length mistmatch")
	}
	for k := range bits {
		bits[k] = blhs.Bits[k] | brhs.Bits[k]
	}
	//XXX intersect nulls
	return blhs.CopyWithBits(bits)
}

// EvalBool evaluates e using val to computs a boolean result.  For elements
// of the result that are not boolean, an error is calculated for each non-bool
// slot and they are returned as an error.  If all of the value slots are errors,
// then the return value is nil.
func EvalBool(zctx *super.Context, val vector.Any, e Evaluator) (vector.Any, bool) {
	//XXX Eval could return a dynamic vector of errors and bools and we should
	// handle this correctly so the logic above is really the fast path
	// and a slower path will handle picking apart the dynamic vector.
	// maybe we could have a generic way to traverse dynamics for
	// appliers doing their thing along the slow path
	if val, ok := vector.Under(e.Eval(val)).(*vector.Bool); ok {
		return val, true
	}
	//XXX need to implement a sparse dynamic (vector.Collection?)
	// and check for that here.
	// for now, if the vector is not uniformly boolean, we return error.
	// XXX example is a field ref a union of structs where the type of
	// the referenced field changes... there can be an arbitrary number
	// of underlying types though any given slot has only one type
	// obviously at any given time.
	return vector.NewStringError(zctx, "not type bool", val.Len()), false
}
