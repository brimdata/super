package agg

import (
	"github.com/brimdata/super"
)

type Count int64

var _ Function = (*Count)(nil)

func (c *Count) Consume(val super.Value) {
	if !val.IsNull() {
		*c++
	}
}

func (c Count) Result(*super.Context) super.Value {
	return super.NewInt64(int64(c))
}

func (c *Count) ConsumeAsPartial(partial super.Value) {
	if partial.Type() != super.TypeInt64 {
		panic("count: bad partial")
	}
	*c += Count(partial.Int())
}

func (c Count) ResultAsPartial(*super.Context) super.Value {
	return c.Result(nil)
}
