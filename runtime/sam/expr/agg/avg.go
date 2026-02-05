package agg

import (
	"errors"
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr/coerce"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/sup"
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
	return super.Null
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

func (a *Avg) ResultAsPartial(sctx *super.Context) super.Value {
	var b scode.Builder
	b.Append(super.EncodeFloat64(a.sum))
	b.Append(super.EncodeUint(a.count))
	typ := sctx.MustLookupTypeRecord([]super.Field{
		super.NewField(sumName, super.TypeFloat64, false),
		super.NewField(countName, super.TypeUint64, false),
	})
	return super.NewValue(typ, b.Bytes())
}
