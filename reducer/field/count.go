package field

import (
	"github.com/mccanne/zq/streamfn"
	"github.com/mccanne/zq/zbuf"
	"github.com/mccanne/zq/zng"
	"github.com/mccanne/zq/zx"
)

type Count struct {
	fn *streamfn.Uint64
}

func NewCountStreamfn(op string) Streamfn {
	return &Count{
		fn: streamfn.NewUint64(op),
	}
}

func (c *Count) Result() zng.Value {
	return zng.NewCount(c.fn.State)
}

func (c *Count) Consume(v zng.Value) error {
	//XXX need CoerceToUint64
	if i, ok := zx.CoerceToInt(v); ok {
		c.fn.Update(uint64(i))
		return nil
	}
	return zbuf.ErrTypeMismatch
}
