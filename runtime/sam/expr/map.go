package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type mapCall struct {
	builder scode.Builder
	eval    Evaluator
	inner   Evaluator
	sctx    *super.Context

	// vals is used to reduce allocations
	vals []super.Value
	// types is used to reduce allocations
	types []super.Type
}

func NewMapCall(sctx *super.Context, e, inner Evaluator) Evaluator {
	return &mapCall{eval: e, inner: inner, sctx: sctx}
}

func (a *mapCall) Eval(in super.Value) super.Value {
	val := a.eval.Eval(in)
	if val.IsError() {
		return val
	}
	elems, err := val.Elements()
	if err != nil {
		return a.sctx.WrapError(err.Error(), in)
	}
	if len(elems) == 0 {
		return val
	}
	a.vals = a.vals[:0]
	a.types = a.types[:0]
	for _, elem := range elems {
		val := a.inner.Eval(elem)
		a.vals = append(a.vals, val)
		a.types = append(a.types, val.Type())
	}
	inner := a.innerType(a.types)
	bytes := a.buildVal(inner)
	if _, ok := super.TypeUnder(val.Type()).(*super.TypeSet); ok {
		return super.NewValue(a.sctx.LookupTypeSet(inner), super.NormalizeSet(bytes))
	}
	return super.NewValue(a.sctx.LookupTypeArray(inner), bytes)
}

func (a *mapCall) buildVal(inner super.Type) []byte {
	a.builder.Reset()
	if union, ok := inner.(*super.TypeUnion); ok {
		for _, val := range a.vals {
			super.BuildUnion(&a.builder, union.TagOf(val.Type()), val.Bytes())
		}
	} else {
		for _, val := range a.vals {
			a.builder.Append(val.Bytes())
		}
	}
	return a.builder.Bytes()
}

func (a *mapCall) innerType(types []super.Type) super.Type {
	types = super.UniqueTypes(types)
	if len(types) == 1 {
		return types[0]
	}
	return a.sctx.LookupTypeUnion(types)
}
