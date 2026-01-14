package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type mapCall struct {
	sctx    *super.Context
	builder scode.Builder
	expr    Evaluator
	lambda  Evaluator
}

func NewMapCall(sctx *super.Context, e, lambda Evaluator) Evaluator {
	return &mapCall{sctx: sctx, expr: e, lambda: lambda}
}

func (m *mapCall) Eval(in vector.Any) vector.Any {
	return vector.Apply(true, m.eval, m.expr.Eval(in))
}

func (m *mapCall) eval(vecs ...vector.Any) vector.Any {
	vec := vector.Under(vecs[0])
	if _, ok := vec.(*vector.Error); ok {
		return vec
	}
	var index []uint32
	if view, ok := vec.(*vector.View); ok {
		index = view.Index
		vec = view.Any
	}
	inner, offsets, nulls, ok := elements(vec)
	if !ok {
		return vector.NewWrappedError(m.sctx, "map: expected array or set value", vecs[0])
	}
	inner = m.lambda.Eval(inner)
	if d, ok := inner.(*vector.Dynamic); ok {
		var typs []super.Type
		for _, vec := range d.Values {
			typs = append(typs, vec.Type())
		}
		utyp := m.sctx.LookupTypeUnion(super.UniqueTypes(typs))
		inner = vector.NewUnion(utyp, d.Tags, d.Values, bitvec.Zero)
	}
	var out vector.Any
	switch vec := vec.(type) {
	case *vector.Array:
		typ := m.sctx.LookupTypeArray(inner.Type())
		out = vector.NewArray(typ, offsets, inner, nulls)
	case *vector.Set:
		typ := m.sctx.LookupTypeSet(inner.Type())
		out = vector.NewSet(typ, offsets, inner, nulls)
	default:
		panic(vec)
	}
	if index != nil {
		out = vector.Pick(out, index)
	}
	return out
}

func elements(vec vector.Any) (vector.Any, []uint32, bitvec.Bits, bool) {
	switch vec := vec.(type) {
	case *vector.Array:
		return vec.Values, vec.Offsets, vec.Nulls, true
	case *vector.Set:
		return vec.Values, vec.Offsets, vec.Nulls, true
	default:
		return nil, nil, bitvec.Zero, false
	}
}
