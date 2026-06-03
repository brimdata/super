package function

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
)

type NullIf struct {
	compare *expr.Compare
}

func newNullIf(sctx *super.Context) *NullIf {
	return &NullIf{expr.NewCompare(sctx, "==", nil, nil)}
}

func (n *NullIf) ApplyOpt() vector.ApplyOpt { return vector.ApplyNone }

func (n *NullIf) Call(vecs ...vector.Any) vector.Any {
	if k := vecs[0].Kind(); k == vector.KindNull || k == vector.KindError {
		return vecs[0]
	}
	if vecs[1].Kind() == vector.KindError {
		return vecs[1]
	}
	bools, _ := expr.BoolMask(vector.Apply(vector.ApplyRipUnions|vector.ApplyRipFusions, func(vecs ...vector.Any) vector.Any {
		return n.compare.Compare(vecs[0], vecs[1])
	}, slices.Clone(vecs)...))
	if bools.IsEmpty() {
		return vecs[0]
	}
	if bools.GetCardinality() == uint64(vecs[0].Len()) {
		return vector.NewNull(vecs[0].Len())
	}
	index := bools.ToArray()
	nullsVec := vector.NewNull(uint32(len(index)))
	return vector.Combine(vector.ReversePick(vecs[0], index), index, nullsVec)
}
