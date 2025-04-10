package function

import (
	"github.com/RoaringBitmap/roaring"
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
)

type Has struct {
	missing Missing
	not     *expr.Not
}

func newHas(sctx *super.Context) *Has {
	return &Has{not: expr.NewLogicalNot(sctx, &expr.This{})}
}

func (h *Has) Call(args ...vector.Any) vector.Any {
	return h.not.Eval(h.missing.Call(args...))
}

type Missing struct{}

func (m *Missing) Call(args ...vector.Any) vector.Any {
	n := args[0].Len()
	var nbm roaring.Bitmap
	for _, vec := range args {
		if nulls := vector.NullsOf(vec); nulls != nil {
			nbm.Or(roaring.FromDense(nulls.Bits, false))
		}
		if err, ok := vec.(*vector.Error); ok {
			b := missingOrQuiet(err)
			if b.IsEmpty() {
				return err
			}
			if b.GetCardinality() == uint64(n) {
				return vector.NewConst(super.True, vec.Len(), bitmapToBool(&nbm, n))
			}
			// Mix of errors and trues.
			index := b.ToArray()
			errIndex := roaring.Flip(b, 0, uint64(n)).ToArray()
			trueVec := vector.NewConst(super.True, uint32(len(index)), nil)
			if !nbm.IsEmpty() {
				trueVec.Nulls = vector.Pick(bitmapToBool(&nbm, n), index).(*vector.Bool)
			}
			return vector.Combine(trueVec, errIndex, vector.Pick(err, errIndex))
		}
	}
	return vector.NewConst(super.False, args[0].Len(), bitmapToBool(&nbm, n))
}

func bitmapToBool(b *roaring.Bitmap, len uint32) *vector.Bool {
	if b.IsEmpty() {
		return nil
	}
	return vector.NewBool(b.ToDense(), len, nil)
}

func missingOrQuiet(verr *vector.Error) *roaring.Bitmap {
	b := roaring.New()
	inner := verr.Vals
	if inner.Type() != super.TypeString {
		return b
	}
	switch inner := inner.(type) {
	case *vector.Const:
		s, _ := inner.AsString()
		if s == "missing" || s == "quiet" {
			b.AddRange(0, uint64(inner.Len()))
		}
	case *vector.View:
		vec := inner.Any.(*vector.String)
		for i := range inner.Len() {
			s := vec.Value(inner.Index[i])
			if s == "missing" || s == "quiet" {
				b.Add(i)
			}
		}
	case *vector.Dict:
		vec := inner.Any.(*vector.String)
		for i := range inner.Len() {
			s := vec.Value(uint32(inner.Index[i]))
			if s == "missing" || s == "quiet" {
				b.Add(i)
			}
		}
	case *vector.String:
		for i := range inner.Len() {
			s := inner.Value(i)
			if s == "missing" || s == "quiet" {
				b.Add(i)
			}
		}
	default:
		panic(inner)
	}
	return b
}
