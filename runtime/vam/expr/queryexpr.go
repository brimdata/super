package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
	"github.com/brimdata/super/zcode"
)

type QueryExpr struct {
	rctx   *runtime.Context
	puller vector.Puller
	cached vector.Any
}

func NewQueryExpr(rctx *runtime.Context, puller vector.Puller) *QueryExpr {
	return &QueryExpr{rctx: rctx, puller: puller}
}

func (q *QueryExpr) Eval(this vector.Any) vector.Any {
	if q.cached == nil {
		q.cached = q.exec(this.Len())
	}
	switch vec := q.cached.(type) {
	case *vector.Const:
		return vector.NewConst(vec.Value(), this.Len(), bitvec.Zero)
	default:
		if this.Len() > 1 {
			// This is an array so just create a view that repeats this.Len().
			return vector.Pick(vec, make([]uint32, this.Len()))
		}
		return vec
	}

}

func (q *QueryExpr) exec(length uint32) vector.Any {
	var vecs []vector.Any
	for {
		vec, err := q.puller.Pull(false)
		if err != nil {
			return vector.NewStringError(q.rctx.Sctx, err.Error(), length)
		}
		if vec == nil {
			return combine(q.rctx.Sctx, vecs)
		}
		vecs = append(vecs, vec)
	}
}

func combine(sctx *super.Context, vecs []vector.Any) vector.Any {
	var b zcode.Builder
	db := vector.NewDynamicBuilder()
	for _, vec := range vecs {
		for i := range vec.Len() {
			var typ super.Type
			if dynamic, ok := vec.(*vector.Dynamic); ok {
				typ = dynamic.TypeOf(i)
			} else {
				typ = vec.Type()
			}
			b.Reset()
			vec.Serialize(&b, i)
			db.Write(super.NewValue(typ, b.Bytes().Body()))
		}
	}
	vec := db.Build()
	switch vec.Len() {
	case 0:
		return vector.NewConst(super.Null, 1, bitvec.Zero)
	case 1:
		return vec
	default:
		return makeArray(sctx, vec)
	}
}

func makeArray(sctx *super.Context, vec vector.Any) vector.Any {
	var typ *super.TypeArray
	if dynamic, ok := vec.(*vector.Dynamic); ok {
		var types []super.Type
		for _, vec := range dynamic.Values {
			types = append(types, vec.Type())
		}
		utyp := sctx.LookupTypeUnion(types)
		typ = sctx.LookupTypeArray(utyp)
		vec = &vector.Union{Dynamic: dynamic, Typ: utyp, Nulls: bitvec.Zero}
	} else {
		typ = sctx.LookupTypeArray(vec.Type())
	}
	offsets := []uint32{0, vec.Len()}
	return vector.NewArray(typ, offsets, vec, bitvec.Zero)
}
