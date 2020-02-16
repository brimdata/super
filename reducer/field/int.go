package field

import (
	"github.com/brimsec/zq/streamfn"
	"github.com/brimsec/zq/zng"
	"github.com/brimsec/zq/zx"
)

type Int struct {
	fn *streamfn.Int64
}

func NewIntStreamfn(op string) Streamfn {
	return &Int{
		fn: streamfn.NewInt64(op),
	}
}

func (i *Int) Result() zng.Value {
	return zng.Value{zng.TypeInt64, zng.EncodeInt(i.fn.State)}
}

func (i *Int) Consume(v zng.Value) error {
	if v, ok := zx.CoerceToInt(v, zng.TypeInt64); ok {
		arg, err := zng.DecodeInt(v.Bytes)
		if err != nil {
			return zng.ErrBadValue
		}
		i.fn.Update(arg)
		return nil
	}
	return zng.ErrTypeMismatch
}
