package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
	"github.com/brimdata/super/zcode"
)

type setExpr struct {
	sctx  *super.Context
	elems []ListElem
}

func NewSetExpr(sctx *super.Context, elems []ListElem) Evaluator {
	return &setExpr{sctx, elems}
}

func (s *setExpr) Eval(this vector.Any) vector.Any {
	var vecs []vector.Any
	for _, e := range s.elems {
		if e.Spread != nil {
			vecs = append(vecs, e.Spread.Eval(this))
		} else {
			vecs = append(vecs, e.Value.Eval(this))
		}
	}
	return vector.Apply(false, s.eval, vecs...)
}

func (a *setExpr) eval(in ...vector.Any) vector.Any {
	offsets, inner := buildList(a.sctx, a.elems, in)
	// Dedupe list elems
	vb := vector.NewBuilder(a.sctx.LookupTypeSet(inner.Type()))
	var b zcode.Builder
	for i := range len(offsets) - 1 {
		b.Truncate()
		for off := offsets[i]; off < offsets[i+1]; off++ {
			inner.Serialize(&b, off)
		}
		vb.Write(super.NormalizeSet(b.Bytes()))
	}
	return vb.Build(bitvec.Zero)
}
