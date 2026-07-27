package agg

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type Any struct {
	result vector.Any
}

func NewAny() *Any {
	return &Any{}
}

func (a *Any) NoRip() bool { return true }

func (a *Any) Consume(vec vector.Any) {
	if a.result != nil {
		return
	}
	slot := firstNonNullSlot(vec)
	if slot != -1 {
		a.result = vector.Pick(vec, []uint32{uint32(slot)})
	}
}

func firstNonNullSlot(vec vector.Any) int {
	if vec.Len() == 0 {
		return -1
	}
	switch vec.Kind() {
	case vector.KindNull:
		return -1
	case vector.KindFusion:
		return firstNonNullSlot(vector.Super(vec))
	case vector.KindUnion:
		union := vec.(*vector.Union)
		for i, vec := range union.Values() {
			if slot := firstNonNullSlot(vec); slot != -1 {
				return int(union.Dynamic().ReverseTagMap()[i][slot])
			}
		}
		return -1
	default:
		return 0
	}
}

func (a *Any) ConsumeAsPartial(vec vector.Any) {
	a.Consume(vec)
}

func (a *Any) Result(sctx *super.Context) vector.Any {
	if a.result == nil {
		return vector.NewNull(1)
	}
	return a.result
}

func (a *Any) ResultAsPartial(*super.Context) vector.Any {
	return a.Result(nil)
}
