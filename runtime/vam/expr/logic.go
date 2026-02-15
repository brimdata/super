package expr

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type Not struct {
	sctx *super.Context
	expr Evaluator
}

var _ Evaluator = (*Not)(nil)

func NewLogicalNot(sctx *super.Context, e Evaluator) *Not {
	return &Not{sctx, e}
}

func (n *Not) Eval(val vector.Any) vector.Any {
	return evalBool(n.sctx, n.eval, n.expr.Eval(val))
}

func (n *Not) eval(vecs ...vector.Any) vector.Any {
	if vecs[0].Kind() == vector.KindNull {
		return vecs[0]
	}
	switch vec := vecs[0].(type) {
	case *vector.Bool:
		return vector.Not(vec)
	case *vector.Const:
		return vector.NewConst(super.NewBool(!vec.Value().Bool()), vec.Len())
	case *vector.Error:
		return vec
	default:
		panic(vec)
	}
}

type And struct {
	sctx *super.Context
	lhs  Evaluator
	rhs  Evaluator
}

func NewLogicalAnd(sctx *super.Context, lhs, rhs Evaluator) *And {
	return &And{sctx, lhs, rhs}
}

func (a *And) Eval(val vector.Any) vector.Any {
	return evalBool(a.sctx, and, a.lhs.Eval(val), a.rhs.Eval(val))
}

func and(vecs ...vector.Any) vector.Any {
	lhs, rhs := vecs[0], vecs[1]
	lhsKind, rhsKind := lhs.Kind(), rhs.Kind()
	switch {
	case lhsKind == vector.KindNull:
		if rhsKind == vector.KindNull {
			return lhs
		}
		if rhsKind == vector.KindError {
			return rhs
		}
		return andErrorOrNull(rhs, lhs)
	case rhsKind == vector.KindNull:
		if lhsKind == vector.KindError {
			return lhs
		}
		return andErrorOrNull(lhs, rhs)
	case lhsKind == vector.KindError:
		if rhsKind == vector.KindError {
			return lhs
		}
		return andErrorOrNull(rhs, lhs)
	case rhsKind == vector.KindError:
		return andErrorOrNull(lhs, rhs)
	}
	blhs, brhs := FlattenBool(lhs), FlattenBool(rhs)
	return vector.NewBool(bitvec.And(blhs.Bits, brhs.Bits))
}

func andErrorOrNull(boolVec, errorOrNullVec vector.Any) vector.Any {
	// true and errorOrNull = errorOrNull
	// false and any = false
	var index []uint32
	for i := range boolVec.Len() {
		if vector.BoolValue(boolVec, i) {
			index = append(index, i)
		}
	}
	return combine(boolVec, errorOrNullVec, index)
}

func combine(baseVec, vec vector.Any, index []uint32) vector.Any {
	if len(index) == 0 {
		return baseVec
	}
	if len(index) == int(vec.Len()) {
		return vec
	}
	baseVec = vector.ReversePick(baseVec, index)
	vec = vector.Pick(vec, index)
	return vector.Combine(baseVec, index, vec)
}

type Or struct {
	sctx *super.Context
	lhs  Evaluator
	rhs  Evaluator
}

func NewLogicalOr(sctx *super.Context, lhs, rhs Evaluator) *Or {
	return &Or{sctx, lhs, rhs}
}

func (o *Or) Eval(val vector.Any) vector.Any {
	return EvalOr(o.sctx, o.lhs.Eval(val), o.rhs.Eval(val))
}

func EvalOr(sctx *super.Context, lhs, rhs vector.Any) vector.Any {
	return evalBool(sctx, or, lhs, rhs)
}

func or(vecs ...vector.Any) vector.Any {
	lhs, rhs := vecs[0], vecs[1]
	switch lhsKind, rhsKind := lhs.Kind(), rhs.Kind(); {
	case lhsKind == vector.KindNull:
		if rhsKind == vector.KindNull || rhsKind == vector.KindError {
			return lhs
		}
		return orErrorOrNull(rhs, lhs)
	case rhsKind == vector.KindNull:
		if lhsKind == vector.KindError {
			return rhs
		}
		return orErrorOrNull(lhs, rhs)
	case lhsKind == vector.KindError:
		if rhsKind == vector.KindError {
			return lhs
		}
		return orErrorOrNull(rhs, lhs)
	case rhsKind == vector.KindError:
		return orErrorOrNull(lhs, rhs)
	}
	return vector.Or(FlattenBool(lhs), FlattenBool(rhs))
}

func orErrorOrNull(boolVec, errorOrNullVec vector.Any) vector.Any {
	// false or errorOrNull = errorOrNull
	// true or any = true
	var index []uint32
	for i := range boolVec.Len() {
		if !vector.BoolValue(boolVec, i) {
			index = append(index, i)
		}
	}
	return combine(boolVec, errorOrNullVec, index)
}

// evalBool evaluates e using val to computs a boolean result.  For elements
// of the result that are not boolean, an error is calculated for each non-bool
// slot and they are returned as an error.  If all of the value slots are errors,
// then the return value is nil.
func evalBool(sctx *super.Context, fn func(...vector.Any) vector.Any, vecs ...vector.Any) vector.Any {
	return vector.Apply(false, func(vecs ...vector.Any) vector.Any {
		for i, vec := range vecs {
			vec := vector.Under(vec)
			if k := vec.Kind(); k == vector.KindBool || k == vector.KindNull || k == vector.KindError {
				vecs[i] = vec
			} else {
				vecs[i] = vector.NewWrappedError(sctx, "not type bool", vec)
			}
		}
		return fn(vecs...)
	}, vecs...)
}

func FlattenBool(vec vector.Any) *vector.Bool {
	switch vec := vec.(type) {
	case *vector.Const:
		val := vec.Value()
		if val.Bool() {
			return vector.NewTrue(vec.Len())
		}
		return vector.NewFalse(vec.Len())
	case *vector.Dynamic:
		out := vector.NewFalse(vec.Len())
		for i := range vec.Len() {
			if vector.BoolValue(vec, i) {
				out.Set(i)
			}
		}
		return out
	case *vector.Bool:
		return vec
	default:
		panic(vec)
	}
}

type In struct {
	sctx *super.Context
	lhs  Evaluator
	rhs  Evaluator
	pw   *PredicateWalk
}

func NewIn(sctx *super.Context, lhs, rhs Evaluator) *In {
	return &In{sctx, lhs, rhs, NewPredicateWalk(NewCompare(sctx, "==", nil, nil).eval)}
}

func (i *In) Eval(this vector.Any) vector.Any {
	return vector.Apply(true, i.eval, i.lhs.Eval(this), i.rhs.Eval(this))
}

func (i *In) eval(vecs ...vector.Any) vector.Any {
	lhs, rhs := vecs[0], vecs[1]
	if k := lhs.Kind(); k == vector.KindNull || k == vector.KindError {
		return lhs
	}
	if rhs.Type().Kind() == super.ErrorKind {
		return rhs
	}
	return i.pw.Eval(lhs, rhs)
}

type PredicateWalk struct {
	pred func(...vector.Any) vector.Any
}

func NewPredicateWalk(pred func(...vector.Any) vector.Any) *PredicateWalk {
	return &PredicateWalk{pred}
}

func (p *PredicateWalk) Eval(vecs ...vector.Any) vector.Any {
	lhs, rhs := vecs[0], vecs[1]
	rhs = vector.Under(rhs)
	rhsOrig := rhs
	var index []uint32
	if view, ok := rhs.(*vector.View); ok {
		rhs = view.Any
		index = view.Index
	}
	switch rhs := rhs.(type) {
	case *vector.Record:
		out := vector.NewFalse(lhs.Len())
		for _, f := range rhs.Fields {
			if index != nil {
				f = vector.Pick(f, index)
			}
			out = vector.Or(out, FlattenBool(p.Eval(lhs, f)))
		}
		return out
	case *vector.Array:
		return p.evalForList(lhs, rhs.Values, rhs.Offsets, index)
	case *vector.Set:
		return p.evalForList(lhs, rhs.Values, rhs.Offsets, index)
	case *vector.Map:
		return vector.Or(p.evalForList(lhs, rhs.Keys, rhs.Offsets, index),
			p.evalForList(lhs, rhs.Values, rhs.Offsets, index))
	case *vector.Union:
		if index != nil {
			panic("vector.Union unexpected in vector.View")
		}
		return vector.Apply(true, p.Eval, lhs, rhs)
	case *vector.Error:
		if index != nil {
			panic("vector.Error unexpected in vector.View")
		}
		return p.Eval(lhs, rhs.Vals)
	default:
		return p.pred(lhs, rhsOrig)
	}
}

func (p *PredicateWalk) evalForList(lhs, rhs vector.Any, offsets, index []uint32) *vector.Bool {
	out := vector.NewFalse(lhs.Len())
	var lhsIndex, rhsIndex []uint32
	for j := range lhs.Len() {
		idx := j
		if index != nil {
			idx = index[j]
		}
		start, end := offsets[idx], offsets[idx+1]
		if start == end {
			continue
		}
		n := end - start
		lhsIndex = slices.Grow(lhsIndex[:0], int(n))[:n]
		rhsIndex = slices.Grow(rhsIndex[:0], int(n))[:n]
		for k := range n {
			lhsIndex[k] = j
			rhsIndex[k] = k + start
		}
		lhsView := vector.Pick(lhs, lhsIndex)
		rhsView := vector.Pick(rhs, rhsIndex)
		b := FlattenBool(p.Eval(lhsView, rhsView))
		if b.Bits.TrueCount() > 0 {
			out.Set(j)
		}
	}
	return out
}
