package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zcode"
)

type ListElem struct {
	Value  Evaluator
	Spread Evaluator
}

type ArrayExpr struct {
	elems []ListElem
	zctx  *super.Context
}

func NewArrayExpr(zctx *super.Context, elems []ListElem) *ArrayExpr {
	return &ArrayExpr{
		elems: elems,
		zctx:  zctx,
	}
}

func (a *ArrayExpr) Eval(this vector.Any) vector.Any {
	var vecs []vector.Any
	for _, e := range a.elems {
		if e.Spread != nil {
			vecs = append(vecs, e.Spread.Eval(this))
		} else {
			vecs = append(vecs, e.Value.Eval(this))
		}
	}
	return vector.Apply(false, a.eval, vecs...)
}

func (a *ArrayExpr) eval(in ...vector.Any) vector.Any {
	n := in[0].Len()
	if n == 0 {
		return vector.NewConst(super.Null, 0, nil)
	}
	var spreadOffs [][]uint32
	var viewIndexes [][]uint32
	var vecs []vector.Any
	for i, elem := range a.elems {
		vec := in[i]
		var offsets, index []uint32
		if elem.Spread != nil {
			vec, offsets, index = a.unwrapSpread(in[i])
			if vec == nil {
				// drop unspreadable elements.
				continue
			}
		}
		vecs = append(vecs, vec)
		spreadOffs = append(spreadOffs, offsets)
		viewIndexes = append(viewIndexes, index)
	}
	offsets := []uint32{0}
	var tags []uint32
	for i := range n {
		var size uint32
		for tag, spreadOff := range spreadOffs {
			if len(spreadOff) == 0 {
				tags = append(tags, uint32(tag))
				size++
			} else {
				if index := viewIndexes[tag]; index != nil {
					i = index[i]
				}
				off := spreadOff[i]
				for end := spreadOff[i+1]; off < end; off++ {
					tags = append(tags, uint32(tag))
					size++
				}
			}
		}
		offsets = append(offsets, offsets[i]+size)
	}
	var typ super.Type
	var innerVec vector.Any
	if len(vecs) == 1 {
		typ = vecs[0].Type()
		innerVec = vecs[0]
	} else {
		var all []super.Type
		for _, vec := range vecs {
			all = append(all, vec.Type())
		}
		types := super.UniqueTypes(all)
		if len(types) == 1 {
			typ = types[0]
			innerVec = mergeSameTypeVecs(typ, tags, vecs)
		} else {
			typ = a.zctx.LookupTypeUnion(types)
			innerVec = vector.NewUnion(typ.(*super.TypeUnion), tags, vecs, nil)
		}
	}
	return vector.NewArray(a.zctx.LookupTypeArray(typ), offsets, innerVec, nil)
}

func (a *ArrayExpr) unwrapSpread(vec vector.Any) (vector.Any, []uint32, []uint32) {
	switch vec := vec.(type) {
	case *vector.Array:
		return vec.Values, vec.Offsets, nil
	case *vector.Set:
		return vec.Values, vec.Offsets, nil
	case *vector.View:
		vals, offsets, _ := a.unwrapSpread(vec.Any)
		return vals, offsets, vec.Index
	}
	return nil, nil, nil
}

func mergeSameTypeVecs(typ super.Type, tags []uint32, vecs []vector.Any) vector.Any {
	// XXX This is going to be slow. At some point would nice to write a native
	// merge of same type vectors.
	counts := make([]uint32, len(vecs))
	vb := vector.NewBuilder(typ)
	var b zcode.Builder
	for _, tag := range tags {
		b.Truncate()
		vecs[tag].Serialize(&b, counts[tag])
		vb.Write(b.Bytes().Body())
		counts[tag]++
	}
	return vb.Build()
}
