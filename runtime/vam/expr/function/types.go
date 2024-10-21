package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#typeof
type TypeOf struct {
	zctx *super.Context
}

func (t *TypeOf) Call(args ...vector.Any) vector.Any {
	val := t.zctx.LookupTypeValue(args[0].Type())
	return vector.NewConst(val, args[0].Len(), nil)
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#kind
type Kind struct {
	zctx *super.Context
}

func NewKind(zctx *super.Context) *Kind {
	return &Kind{zctx}
}

func (k *Kind) Call(args ...vector.Any) vector.Any {
	vec := vector.Under(args[0])
	if typ := vec.Type(); typ.ID() != super.IDType {
		s := typ.Kind().String()
		return vector.NewConst(super.NewString(s), vec.Len(), nil)
	}
	out := vector.NewStringEmpty(vec.Len(), nil)
	for i, n := uint32(0), vec.Len(); i < n; i++ {
		var s string
		if bytes, null := vector.TypeValueValue(vec, i); !null {
			typ, err := k.zctx.LookupByValue(bytes)
			if err != nil {
				panic(err)
			}
			s = typ.Kind().String()
		}
		out.Append(s)
	}
	return out
}

func (*Kind) RipUnions() bool {
	return false
}
