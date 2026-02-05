package agg

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
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

func (a *avg) Result(*super.Context) super.Value {
	if a.count > 0 {
		return super.NewFloat64(a.sum / float64(a.count))
	}
	return super.Null
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
	fields := rec.Fields(nil)
	sumVal := fields[si]
	countVal := fields[ci]
	if sumVal.Type() != super.TypeFloat64 || countVal.Type() != super.TypeUint64 {
		panic("avg: invalid partial")
	}
	a.sum += vector.FloatValue(sumVal, idx)
	a.count += vector.UintValue(countVal, idx)
}

func (a *avg) ResultAsPartial(sctx *super.Context) super.Value {
	var b scode.Builder
	b.Append(super.EncodeFloat64(a.sum))
	b.Append(super.EncodeUint(a.count))
	typ := sctx.MustLookupTypeRecord([]super.Field{
		super.NewField(sumName, super.TypeFloat64, false),
		super.NewField(countName, super.TypeUint64, false),
	})
	return super.NewValue(typ, b.Bytes())
}
