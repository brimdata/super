package expr

import (
	"github.com/RoaringBitmap/roaring/v2"
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type conditional struct {
	sctx      *super.Context
	predicate Evaluator
	thenExpr  Evaluator
	elseExpr  Evaluator
}

func NewConditional(sctx *super.Context, predicate, thenExpr, elseExpr Evaluator) Evaluator {
	return &conditional{
		sctx:      sctx,
		predicate: predicate,
		thenExpr:  thenExpr,
		elseExpr:  elseExpr,
	}
}

func (c *conditional) Eval(this vector.Any) vector.Any {
	n := uint64(this.Len())
	pred := c.predicate.Eval(this)
	trues, _, other := BoolMask(pred)
	if trues.GetCardinality() == n {
		return c.thenExpr.Eval(this)
	}
	if other.GetCardinality() == n {
		return c.errPredicateType(pred)
	}
	if trues.IsEmpty() && other.IsEmpty() {
		return c.elseExpr.Eval(this)
	}
	falses := roaring.Flip(roaring.Or(trues, other), 0, n)
	var vecs []vector.Any
	tags := make([]uint32, n)
	if !trues.IsEmpty() {
		index := trues.ToArray()
		for _, idx := range index {
			tags[idx] = uint32(len(vecs))
		}
		vecs = append(vecs, c.thenExpr.Eval(vector.Pick(this, index)))
	}
	if !falses.IsEmpty() {
		index := falses.ToArray()
		for _, idx := range index {
			tags[idx] = uint32(len(vecs))
		}
		vecs = append(vecs, c.elseExpr.Eval(vector.Pick(this, index)))
	}
	if !other.IsEmpty() {
		index := other.ToArray()
		for _, idx := range index {
			tags[idx] = uint32(len(vecs))
		}
		vecs = append(vecs, c.errPredicateType(vector.Pick(pred, index)))
	}
	return vector.NewDynamic(tags, vecs)
}

func (c *conditional) errPredicateType(pred vector.Any) vector.Any {
	return vector.Apply(true, func(vecs ...vector.Any) vector.Any {
		vec := vecs[0]
		if vec.Kind() == vector.KindError {
			return vec
		}
		return vector.NewWrappedError(c.sctx, "?-operator: bool predicate required", vec)
	}, pred)
}

func BoolMask(mask vector.Any) (*roaring.Bitmap, *roaring.Bitmap, *roaring.Bitmap) {
	mask = vector.Apply(true, func(vecs ...vector.Any) vector.Any {
		return vecs[0]
	}, mask)
	bools := roaring.New()
	nulls := roaring.New()
	other := roaring.New()
	if dynamic, ok := mask.(*vector.Dynamic); ok {
		reverse := dynamic.ReverseTagMap()
		for i, val := range dynamic.Values {
			boolMaskRidx(reverse[i], bools, nulls, other, val)
		}
	} else {
		boolMaskRidx(nil, bools, nulls, other, mask)
	}
	return bools, nulls, other
}

func boolMaskRidx(ridx []uint32, bools, nulls, other *roaring.Bitmap, vec vector.Any) {
	switch vec := vector.Under(vec).(type) {
	case *vector.Const:
		if vec.Type().ID() != super.IDBool {
			if ridx != nil {
				other.AddMany(ridx)
			} else {
				other.AddRange(0, uint64(vec.Len()))
			}
			return
		}
		if !vector.BoolValue(vec, 0) {
			return
		}
		if ridx != nil {
			bools.AddMany(ridx)
		} else {
			bools.AddRange(0, uint64(vec.Len()))
		}
	case *vector.Bool:
		trues := vec.Bits
		if ridx != nil {
			for i, idx := range ridx {
				if trues.IsSetDirect(uint32(i)) {
					bools.Add(idx)
				}
			}
		} else {
			bools.Or(roaring.FromDense(trues.GetBits(), true))
		}
	case *vector.Null:
		if ridx != nil {
			nulls.AddMany(ridx)
		} else {
			nulls.AddRange(0, uint64(vec.Len()))
		}
	default:
		if ridx != nil {
			other.AddMany(ridx)
		} else {
			other.AddRange(0, uint64(vec.Len()))
		}
	}
}
