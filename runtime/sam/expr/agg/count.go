package agg

import (
	"github.com/brimdata/super"
)

type Count uint64

var _ Function = (*Count)(nil)

func (c *Count) Consume(super.Value) {
	*c++
}

func (c Count) Result(*super.Context) super.Value {
	return super.NewUint64(uint64(c))
}

func (c *Count) ConsumeAsPartial(partial super.Value) {
	if partial.Type() != super.TypeUint64 {
		panic("count: partial not uint64")
	}
	*c += Count(partial.Uint())
}

func (c Count) ResultAsPartial(*super.Context) super.Value {
	return c.Result(nil)
}
