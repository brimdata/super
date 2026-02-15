package function

import (
	"github.com/RoaringBitmap/roaring/v2"
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
	if vec, ok := expr.CheckNulls(args); ok {
		return vec
	}
	n := args[0].Len()
	for _, vec := range args {
		if err, ok := vec.(*vector.Error); ok {
			b := missingOrQuiet(err)
			if b.IsEmpty() {
				return err
			}
			if b.GetCardinality() == uint64(n) {
				return vector.NewConst(super.True, vec.Len())
			}
			// Mix of errors and trues.
			index := b.ToArray()
			errIndex := roaring.Flip(b, 0, uint64(n)).ToArray()
			trueVec := vector.NewConst(super.True, uint32(len(index)))
			return vector.Combine(trueVec, errIndex, vector.Pick(err, errIndex))
		}
	}
	return vector.NewConst(super.False, args[0].Len())
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
