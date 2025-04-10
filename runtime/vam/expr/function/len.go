package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr/function"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#len
type Len struct {
	sctx *super.Context
}

func (l *Len) Call(args ...vector.Any) vector.Any {
	val := vector.Under(args[0])
	out := vector.NewIntEmpty(super.TypeInt64, val.Len(), bitvec.Zero)
	switch typ := val.Type().(type) {
	case *super.TypeOfNull:
		return vector.NewConst(super.NewInt64(0), val.Len(), bitvec.Zero)
	case *super.TypeRecord:
		length := int64(len(typ.Fields))
		return vector.NewConst(super.NewInt64(length), val.Len(), bitvec.Zero)
	case *super.TypeArray, *super.TypeSet, *super.TypeMap:
		for i := uint32(0); i < val.Len(); i++ {
			start, end, _ := vector.ContainerOffset(val, i)
			out.Append(int64(end) - int64(start))
		}
	case *super.TypeOfString:
		for i := uint32(0); i < val.Len(); i++ {
			s, _ := vector.StringValue(val, i)
			out.Append(int64(len(s)))
		}
	case *super.TypeOfBytes:
		for i := uint32(0); i < val.Len(); i++ {
			s, _ := vector.BytesValue(val, i)
			out.Append(int64(len(s)))
		}
	case *super.TypeOfIP:
		for i := uint32(0); i < val.Len(); i++ {
			ip, null := vector.IPValue(val, i)
			if null {
				out.Append(0)
				continue
			}
			out.Append(int64(len(ip.AsSlice())))
		}
	case *super.TypeOfNet:
		for i := uint32(0); i < val.Len(); i++ {
			n, null := vector.NetValue(val, i)
			if null {
				out.Append(0)
				continue
			}
			out.Append(int64(len(super.AppendNet(nil, n))))
		}
	case *super.TypeError:
		return vector.NewWrappedError(l.sctx, "len()", val)
	case *super.TypeOfType:
		for i := uint32(0); i < val.Len(); i++ {
			v, null := vector.TypeValueValue(val, i)
			if null {
				out.Append(0)
				continue
			}
			t, err := l.sctx.LookupByValue(v)
			if err != nil {
				panic(err)
			}
			out.Append(int64(function.TypeLength(t)))
		}
	default:
		return vector.NewWrappedError(l.sctx, "len: bad type", val)
	}
	return out
}
