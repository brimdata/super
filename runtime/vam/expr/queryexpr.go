package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
	"github.com/brimdata/super/zcode"
)

type QueryExpr struct {
	rctx       *runtime.Context
	puller     vector.Puller
	cached     vector.Any
	forceArray bool
}

func NewQueryExpr(rctx *runtime.Context, puller vector.Puller, forceArray bool) *QueryExpr {
	return &QueryExpr{rctx: rctx, puller: puller, forceArray: forceArray}
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
			out := flattenVecs(vecs)
			if q.forceArray {
				return makeArray(q.rctx.Sctx, out)
			}
			switch out.Len() {
			case 0:
				return vector.NewConst(super.Null, 1, bitvec.Zero)
			case 1:
				return out
			default:
				return makeArray(q.rctx.Sctx, out)
			}
		}
		vecs = append(vecs, vec)
	}
}

func flattenVecs(vecs []vector.Any) vector.Any {
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
			b.Truncate()
			vec.Serialize(&b, i)
			db.Write(super.NewValue(typ, b.Bytes().Body()))
		}
	}
	return db.Build()
}

func makeArray(sctx *super.Context, vec vector.Any) vector.Any {
	if vec.Len() == 0 {
		typ := sctx.LookupTypeArray(super.TypeNull)
		return vector.NewArray(typ, []uint32{0, 0}, vector.NewConst(super.Null, 0, bitvec.Zero), bitvec.Zero)
	}
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
