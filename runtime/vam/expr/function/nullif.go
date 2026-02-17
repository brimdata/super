package function

import (
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

func (n *NullIf) Call(vecs ...vector.Any) vector.Any {
	if k := vecs[0].Kind(); k == vector.KindNull || k == vector.KindError {
		return vecs[0]
	}
	if vecs[1].Kind() == vector.KindError {
		return vecs[1]
	}
	result := n.compare.Compare(vecs[0], vecs[1])
	if result.Type().Kind() == super.ErrorKind {
		return vecs[0]
	}
	var index []uint32
	for i := range result.Len() {
		if vector.BoolValue(result, i) {
			index = append(index, i)
		}
	}
	if len(index) == 0 {
		return vecs[0]
	}
	nullsVec := vector.NewConst(super.Null, uint32(len(index)))
	if len(index) == int(vecs[0].Len()) {
		return nullsVec
	}
	return vector.Combine(vector.ReversePick(vecs[0], index), index, nullsVec)
}
