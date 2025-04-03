package agg

import (
	"errors"
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr/coerce"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zcode"
)

type Avg struct {
	sum   float64
	count uint64
}

var _ Function = (*Avg)(nil)

func (a *Avg) Consume(val super.Value) {
	if val.IsNull() {
		return
	}
	if d, ok := coerce.ToFloat(val, super.TypeFloat64); ok {
		a.sum += float64(d)
		a.count++
	}
}

func (a *Avg) Result(*super.Context) super.Value {
	if a.count > 0 {
		return super.NewFloat64(a.sum / float64(a.count))
	}
	return super.NullFloat64
}

const (
	sumName   = "sum"
	countName = "count"
)

func (a *Avg) ConsumeAsPartial(partial super.Value) {
	sumVal := partial.Deref(sumName)
	if sumVal == nil {
		panic(errors.New("avg: partial sum is missing"))
	}
	if sumVal.Type() != super.TypeFloat64 {
		panic(fmt.Errorf("avg: partial sum has bad type: %s", sup.FormatValue(*sumVal)))
	}
	countVal := partial.Deref(countName)
	if countVal == nil {
		panic("avg: partial count is missing")
	}
	if countVal.Type() != super.TypeUint64 {
		panic(fmt.Errorf("avg: partial count has bad type: %s", sup.FormatValue(*countVal)))
	}
	a.sum += sumVal.Float()
	a.count += countVal.Uint()
}

func (a *Avg) ResultAsPartial(zctx *super.Context) super.Value {
	var zv zcode.Bytes
	zv = super.NewFloat64(a.sum).Encode(zv)
	zv = super.NewUint64(a.count).Encode(zv)
	typ := zctx.MustLookupTypeRecord([]super.Field{
		super.NewField(sumName, super.TypeFloat64),
		super.NewField(countName, super.TypeUint64),
	})
	return super.NewValue(typ, zv)
}
