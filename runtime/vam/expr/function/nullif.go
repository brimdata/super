package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type NullIf struct {
	compare *expr.Compare
}

func newNullIf(sctx *super.Context) *NullIf {
	return &NullIf{expr.NewCompare(sctx, "==", nil, nil)}
}

func (n *NullIf) Call(vecs ...vector.Any) vector.Any {
	if vecs[0].Type().Kind() == super.ErrorKind {
		return vecs[0]
	}
	if vecs[1].Type().Kind() == super.ErrorKind {
		return vecs[1]
	}
	result := n.compare.Compare(vecs[0], vecs[1])
	if result.Type().Kind() == super.ErrorKind {
		return vecs[0]
	}
	nulls := vector.NullsOf(vecs[0]).Clone()
	for i := range result.Len() {
		if b, _ := vector.BoolValue(result, i); b {
			if nulls.IsZero() {
				nulls = bitvec.NewFalse(vecs[0].Len())
			}
			nulls.Set(i)
		}
	}
	return vector.CopyAndSetNulls(vecs[0], nulls)
}
