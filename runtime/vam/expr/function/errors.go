package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type Quiet struct {
	sctx *super.Context
}

func (q *Quiet) Call(args ...vector.Any) vector.Any {
	arg, ok := vector.Opt(args[0]).(*vector.Error)
	if !ok {
		return args[0]
	}
	if _, ok := arg.Vals.Type().(*super.TypeOfString); !ok {
		return args[0]
	}
	if _, ok := arg.Vals.(*vector.Const); ok {
		// Fast path
		if vector.StringValue(arg.Vals, 0) == "missing" {
			return vector.NewStringError(q.sctx, "quiet", arg.Len())
		}
		return args[0]
	}
	n := arg.Len()
	vec := vector.NewStringEmpty(n)
	for i := uint32(0); i < n; i++ {
		s := vector.StringValue(arg.Vals, i)
		if s == "missing" {
			s = "quiet"
		}
		vec.Append(s)
	}
	return vector.NewError(arg.Typ, vec)
}
