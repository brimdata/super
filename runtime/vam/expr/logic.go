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
	switch vec := vecs[0].(type) {
	case *vector.Bool:
		return vector.NewBool(bitvec.Not(vec.Bits), vec.Nulls)
	case *vector.Const:
		return vector.NewConst(super.NewBool(!vec.Value().Bool()), vec.Len(), vec.Nulls)
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

type Or struct {
	sctx *super.Context
	lhs  Evaluator
	rhs  Evaluator
}

func NewLogicalOr(sctx *super.Context, lhs, rhs Evaluator) *Or {
	return &Or{sctx, lhs, rhs}
}

func (a *And) Eval(val vector.Any) vector.Any {
	return evalBool(a.sctx, a.eval, a.lhs.Eval(val), a.rhs.Eval(val))
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
	and := bitvec.And(blhs.Bits, brhs.Bits)
	if blhs.Nulls.IsZero() && brhs.Nulls.IsZero() {
		return vector.NewBool(and, bitvec.Zero)
	}
	// any and false = false
	// null and true = null
	notfalse := bitvec.And(bitvec.Or(blhs.Bits, blhs.Nulls), bitvec.Or(brhs.Bits, brhs.Nulls))
	nulls := bitvec.And(notfalse, bitvec.Or(blhs.Nulls, brhs.Nulls))
	return vector.NewBool(and, nulls)
}

func (a *And) andError(err vector.Any, vec vector.Any) vector.Any {
	if _, ok := vec.(*vector.Error); ok {
		return err
	}
	b := toBool(vec)
	// anything and FALSE = FALSE
	isError := bitvec.Or(b.Bits, b.Nulls)
	var index []uint32
	for i := range err.Len() {
		if isError.IsSetDirect(i) {
			index = append(index, i)
		}
	}
	if len(index) > 0 {
		base := vector.ReversePick(vec, index)
		return vector.Combine(base, index, vector.Pick(err, index))
	}
	return vec
}

func (o *Or) Eval(val vector.Any) vector.Any {
	return evalBool(o.sctx, o.eval, o.lhs.Eval(val), o.rhs.Eval(val))
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
	bits := bitvec.Or(blhs.Bits, brhs.Bits)
	if blhs.Nulls.IsZero() && brhs.Nulls.IsZero() {
		// Fast path involves no nulls.
		return vector.NewBool(bits, bitvec.Zero)
	}
	nulls := bitvec.Or(blhs.Nulls, brhs.Nulls)
	nulls = bitvec.And(bitvec.Not(bits), nulls)
	return vector.NewBool(bits, nulls)
}

func (o *Or) orError(err, vec vector.Any) vector.Any {
	if _, ok := vec.(*vector.Error); ok {
		return err
	}
	b := toBool(vec)
	// not error if true or null
	notError := bitvec.Or(b.Bits, b.Nulls)
	var index []uint32
	for i := range b.Len() {
		if !notError.IsSetDirect(i) {
			index = append(index, i)
		}
	}
	if len(index) > 0 {
		base := vector.ReversePick(vec, index)
		return vector.Combine(base, index, vector.Pick(err, index))
	}
	return vec
}

// evalBool evaluates e using val to computs a boolean result.  For elements
// of the result that are not boolean, an error is calculated for each non-bool
// slot and they are returned as an error.  If all of the value slots are errors,
// then the return value is nil.
func evalBool(sctx *super.Context, fn func(...vector.Any) vector.Any, vecs ...vector.Any) vector.Any {
	return vector.Apply(false, func(vecs ...vector.Any) vector.Any {
		for i, vec := range vecs {
			if vec := vector.Under(vec); vec.Type() == super.TypeBool || vector.KindOf(vec) == vector.KindError {
				vecs[i] = vec
			} else {
				vecs[i] = vector.NewWrappedError(sctx, "not type bool", vec)
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
			out := vector.NewTrue(vec.Len())
			out.Nulls = vec.Nulls
			return out
		} else {
			return vector.NewBoolEmpty(vec.Len(), vec.Nulls)
		}
	case *vector.Dynamic:
		nulls := bitvec.NewFalse(vec.Len())
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

type In struct {
	sctx *super.Context
	lhs  Evaluator
	rhs  Evaluator
	pw   *PredicateWalk
}

func NewIn(sctx *super.Context, lhs, rhs Evaluator) *In {
	return &In{sctx, lhs, rhs, NewPredicateWalk(NewCompare(sctx, nil, nil, "==").eval)}
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
		out := vector.NewFalse(lhs.Len())
		for _, f := range rhs.Fields {
			if index != nil {
				f = vector.Pick(f, index)
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
		if toBool(p.Eval(lhsView, rhsView)).Bits.TrueCount() > 0 {
			out.Set(j)
		}
	}
	return out
}
