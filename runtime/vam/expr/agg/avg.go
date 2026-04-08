package agg

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type avg struct {
	sum   float64
	count uint64
}

var _ Func = (*avg)(nil)

func (a *avg) Consume(vec vector.Any) {
	vec = vector.Under(vec)
	if !super.IsNumber(vec.Type().ID()) {
		return
	}
	a.count += uint64(vec.Len())
	a.sum = sum(a.sum, vec)
}

func (a *avg) Result(*super.Context) vector.Any {
	if a.count > 0 {
		f := a.sum / float64(a.count)
		return vector.NewFloat(super.TypeFloat64, []float64{f})
	}
	return vector.NewNull(1)
}

const (
	sumName   = "sum"
	countName = "count"
)

func (a *avg) ConsumeAsPartial(partial vector.Any) {
	if partial.Len() != 1 {
		panic("avg: invalid partial")
	}
	idx := uint32(0)
	if view, ok := partial.(*vector.View); ok {
		idx = view.Index[0]
		partial = view.Any
	}
	rec, ok := partial.(*vector.Record)
	if !ok {
		panic("avg: invalid partial")
	}
	si, ok1 := rec.Typ.IndexOfField(sumName)
	ci, ok2 := rec.Typ.IndexOfField(countName)
	if !ok1 || !ok2 {
		panic("avg: invalid partial")
	}
	fields := rec.Fields
	sumVal := fields[si]
	countVal := fields[ci]
	if sumVal.Type() != super.TypeFloat64 || countVal.Type() != super.TypeUint64 {
		panic("avg: invalid partial")
	}
	a.sum += vector.FloatValue(sumVal, idx)
	a.count += vector.UintValue(countVal, idx)
}

func (a *avg) ResultAsPartial(sctx *super.Context) vector.Any {
	sum := vector.NewFloat(super.TypeFloat64, []float64{a.sum})
	count := vector.NewUint(super.TypeUint64, []uint64{a.count})
	typ := sctx.MustLookupTypeRecord([]super.Field{
		super.NewField(sumName, super.TypeFloat64),
		super.NewField(countName, super.TypeUint64),
	})
	return vector.NewRecord(typ, []vector.Any{sum, count}, 1)
}
