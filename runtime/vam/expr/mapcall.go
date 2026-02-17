package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector"
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
	if vec, ok := CheckForNullThenError(vecs); ok {
		return vec
	}
	vec := vector.Under(vecs[0])
	var index []uint32
	if view, ok := vec.(*vector.View); ok {
		index = view.Index
		vec = view.Any
	}
	inner, offsets, ok := elements(vec)
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
		inner = vector.NewUnion(utyp, d.Tags, d.Values)
	}
	var out vector.Any
	switch vec := vec.(type) {
	case *vector.Array:
		typ := m.sctx.LookupTypeArray(inner.Type())
		out = vector.NewArray(typ, offsets, inner)
	case *vector.Set:
		typ := m.sctx.LookupTypeSet(inner.Type())
		out = vector.NewSet(typ, offsets, inner)
	default:
		panic(vec)
	}
	if index != nil {
		out = vector.Pick(out, index)
	}
	return out
}

func elements(vec vector.Any) (vector.Any, []uint32, bool) {
	switch vec := vec.(type) {
	case *vector.Array:
		return vec.Values, vec.Offsets, true
	case *vector.Set:
		return vec.Values, vec.Offsets, true
	default:
		return nil, nil, false
	}
}
