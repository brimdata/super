package agg

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zcode"
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
	var ncount uint32
	if nulls := vector.NullsOf(vec); nulls != nil {
		ncount = nulls.TrueCount()
	}
	if ncount != vec.Len() {
		a.count += uint64(vec.Len() - ncount)
		a.sum = sum(a.sum, vec)
	}
}

func (a *avg) Result(*super.Context) super.Value {
	if a.count > 0 {
		return super.NewFloat64(a.sum / float64(a.count))
	}
	return super.NullFloat64
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
	sumVal := rec.Fields[si]
	countVal := rec.Fields[ci]
	if sumVal.Type() != super.TypeFloat64 || countVal.Type() != super.TypeUint64 {
		panic("avg: invalid partial")
	}
	sum, _ := vector.FloatValue(sumVal, idx)
	count, _ := vector.UintValue(countVal, idx)
	a.sum += sum
	a.count += count
}

func (a *avg) ResultAsPartial(sctx *super.Context) super.Value {
	var zv zcode.Bytes
	zv = super.NewFloat64(a.sum).Encode(zv)
	zv = super.NewUint64(a.count).Encode(zv)
	typ := sctx.MustLookupTypeRecord([]super.Field{
		super.NewField(sumName, super.TypeFloat64),
		super.NewField(countName, super.TypeUint64),
	})
	return super.NewValue(typ, zv)
}
