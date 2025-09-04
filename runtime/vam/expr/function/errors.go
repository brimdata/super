package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type Quiet struct {
	sctx *super.Context
}

func (q *Quiet) Call(args ...vector.Any) vector.Any {
	arg, ok := args[0].(*vector.Error)
	if !ok {
		return args[0]
	}
	if _, ok := arg.Vals.Type().(*super.TypeOfString); !ok {
		return args[0]
	}
	if c, ok := arg.Vals.(*vector.Const); ok {
		// Fast path
		if s, _ := c.AsString(); s == "missing" {
			return vector.NewStringError(q.sctx, "quiet", c.Len())
		}
		return args[0]
	}
	n := arg.Len()
	vec := vector.NewStringEmpty(n, bitvec.NewFalse(n))
	for i := uint32(0); i < n; i++ {
		s, null := vector.StringValue(arg.Vals, i)
		if null {
			vec.Nulls.Set(i)
		}
		if s == "missing" {
			s = "quiet"
		}
		vec.Append(s)
	}
	return vector.NewError(arg.Typ, vec, arg.Nulls)
}
