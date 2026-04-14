package function

import (
	"unicode/utf8"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr/function"
	"github.com/brimdata/super/vector"
)

type Len struct {
	sctx *super.Context
}

func (l *Len) Call(args ...vector.Any) vector.Any {
	val := vector.Under(args[0])
	out := vector.NewIntEmpty(super.TypeInt64, val.Len())
	switch typ := val.Type().(type) {
	case *super.TypeOfNull:
		return vector.NewConstInt(super.TypeInt64, 0, val.Len())
	case *super.TypeRecord:
		length := int64(len(typ.Fields))
		return vector.NewConstInt(super.TypeInt64, length, val.Len())
	case *super.TypeArray, *super.TypeSet, *super.TypeMap:
		for i := uint32(0); i < val.Len(); i++ {
			start, end := vector.ContainerOffset(val, i)
			out.Append(int64(end) - int64(start))
		}
	case *super.TypeOfString:
		for i := uint32(0); i < val.Len(); i++ {
			s := vector.StringValue(val, i)
			out.Append(int64(utf8.RuneCountInString(s)))
		}
	case *super.TypeOfBytes:
		for i := uint32(0); i < val.Len(); i++ {
			b := vector.BytesValue(val, i)
			out.Append(int64(len(b)))
		}
	case *super.TypeOfIP:
		for i := uint32(0); i < val.Len(); i++ {
			ip := vector.IPValue(val, i)
			out.Append(int64(len(ip.AsSlice())))
		}
	case *super.TypeOfNet:
		for i := uint32(0); i < val.Len(); i++ {
			net := vector.NetValue(val, i)
			out.Append(int64(len(super.AppendNet(nil, net))))
		}
	case *super.TypeError:
		return vector.NewWrappedError(l.sctx, "len()", val)
	case *super.TypeOfType:
		for i := uint32(0); i < val.Len(); i++ {
			typ := vector.TypeValueValue(val, i)
			out.Append(int64(function.TypeLength(typ)))
		}
	default:
		return vector.NewWrappedError(l.sctx, "len: bad type", val)
	}
	return out
}
