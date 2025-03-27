package expr

import (
	"slices"

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
	return evalBool(n.zctx, n.eval, n.expr.Eval(val))
}

func (n *Not) eval(vecs ...vector.Any) vector.Any {
	switch vec := vecs[0].(type) {
	case *vector.Bool:
		bits := make([]uint64, len(vec.Bits))
		for k := range bits {
			bits[k] = ^vec.Bits[k]
		}
		return vec.CopyWithBits(bits)
	case *vector.Const:
		return vector.NewConst(super.NewBool(!vec.Value().Bool()), vec.Len(), vec.Nulls)
	case *vector.Error:
		return vec
	default:
		panic(vec)
	}
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
	return evalBool(a.zctx, a.eval, a.lhs.Eval(val), a.rhs.Eval(val))
}

func (a *And) eval(vecs ...vector.Any) vector.Any {
	if vecs[0].Len() == 0 {
		return vecs[0]
	}
	lhs, rhs := vector.Under(vecs[0]), vector.Under(vecs[1])
	if _, ok := lhs.(*vector.Error); ok {
		return a.andError(lhs, rhs)
	}
	if _, ok := rhs.(*vector.Error); ok {
		return a.andError(rhs, lhs)
	}
	blhs, brhs := toBool(lhs), toBool(rhs)
	out := vector.And(blhs, brhs)
	if blhs.Nulls == nil && brhs.Nulls == nil {
		return out
	}
	// any and false = false
	// null and true = null
	notfalse := vector.And(vector.Or(blhs, blhs.Nulls), vector.Or(brhs, brhs.Nulls))
	out.Nulls = vector.And(notfalse, vector.Or(blhs.Nulls, brhs.Nulls))
	return out
}

func (a *And) andError(err vector.Any, vec vector.Any) vector.Any {
	if _, ok := vec.(*vector.Error); ok {
		return err
	}
	b := toBool(vec)
	// anything and FALSE = FALSE
	isError := vector.Or(b, b.Nulls)
	var index []uint32
	for i := range err.Len() {
		if isError.Value(i) {
			index = append(index, i)
		}
	}
	if len(index) > 0 {
		base := vector.NewInverseView(vec, index)
		return vector.Combine(base, index, vector.NewView(err, index))
	}
	return vec
}

func (o *Or) Eval(val vector.Any) vector.Any {
	return evalBool(o.zctx, o.eval, o.lhs.Eval(val), o.rhs.Eval(val))
}

func (o *Or) eval(vecs ...vector.Any) vector.Any {
	if vecs[0].Len() == 0 {
		return vecs[0]
	}
	lhs, rhs := vector.Under(vecs[0]), vector.Under(vecs[1])
	if _, ok := lhs.(*vector.Error); ok {
		return o.orError(lhs, rhs)
	}
	if _, ok := rhs.(*vector.Error); ok {
		return o.orError(rhs, lhs)
	}
	blhs, brhs := toBool(lhs), toBool(rhs)
	out := vector.Or(blhs, brhs)
	if blhs.Nulls == nil && brhs.Nulls == nil {
		return out
	}
	nulls := vector.Or(blhs.Nulls, brhs.Nulls)
	out.Nulls = vector.And(vector.Not(out), nulls)
	return out
}

func (o *Or) orError(err, vec vector.Any) vector.Any {
	if _, ok := vec.(*vector.Error); ok {
		return err
	}
	b := toBool(vec)
	// not error if true or null
	notError := vector.Or(b, b.Nulls)
	var index []uint32
	for i := range b.Len() {
		if !notError.Value(i) {
			index = append(index, i)
		}
	}
	if len(index) > 0 {
		base := vector.NewInverseView(vec, index)
		return vector.Combine(base, index, vector.NewView(err, index))
	}
	return vec
}

// evalBool evaluates e using val to computs a boolean result.  For elements
// of the result that are not boolean, an error is calculated for each non-bool
// slot and they are returned as an error.  If all of the value slots are errors,
// then the return value is nil.
func evalBool(zctx *super.Context, fn func(...vector.Any) vector.Any, vecs ...vector.Any) vector.Any {
	return vector.Apply(false, func(vecs ...vector.Any) vector.Any {
		for i, vec := range vecs {
			if vec := vector.Under(vec); vec.Type() == super.TypeBool || vector.KindOf(vec) == vector.KindError {
				vecs[i] = vec
			} else {
				vecs[i] = vector.NewWrappedError(zctx, "not type bool", vec)
			}
		}
		return fn(vecs...)
	}, vecs...)
}

func toBool(vec vector.Any) *vector.Bool {
	switch vec := vec.(type) {
	case *vector.Const:
		val := vec.Value()
		if val.Bool() {
			out := trueBool(vec.Len())
			out.Nulls = vec.Nulls
			return out
		} else {
			return vector.NewBoolEmpty(vec.Len(), vec.Nulls)
		}
	case *vector.Dynamic:
		nulls := vector.NewBoolEmpty(vec.Len(), nil)
		out := vector.NewBoolEmpty(vec.Len(), nulls)
		for i := range vec.Len() {
			v, null := vector.BoolValue(vec, i)
			if null {
				nulls.Set(i)
			} else if v {
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

func trueBool(n uint32) *vector.Bool {
	vec := vector.NewBoolEmpty(n, nil)
	for i := range vec.Bits {
		vec.Bits[i] = ^uint64(0)
	}
	return vec
}

type In struct {
	zctx *super.Context
	lhs  Evaluator
	rhs  Evaluator
	pw   *PredicateWalk
}

func NewIn(zctx *super.Context, lhs, rhs Evaluator) *In {
	return &In{zctx, lhs, rhs, NewPredicateWalk(NewCompare(zctx, nil, nil, "==").eval)}
}

func (i *In) Eval(this vector.Any) vector.Any {
	return vector.Apply(true, i.eval, i.lhs.Eval(this), i.rhs.Eval(this))
}

func (i *In) eval(vecs ...vector.Any) vector.Any {
	lhs, rhs := vecs[0], vecs[1]
	if lhs.Type().Kind() == super.ErrorKind {
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
		out := vector.NewBoolEmpty(lhs.Len(), nil)
		for _, f := range rhs.Fields {
			if index != nil {
				f = vector.NewView(f, index)
			}
			out = vector.Or(out, toBool(p.Eval(lhs, f)))
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
	out := vector.NewBoolEmpty(lhs.Len(), nil)
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
		lhsView := vector.NewView(lhs, lhsIndex)
		rhsView := vector.NewView(rhs, rhsIndex)
		if toBool(p.Eval(lhsView, rhsView)).TrueCount() > 0 {
			out.Set(j)
		}
	}
	return out
}
