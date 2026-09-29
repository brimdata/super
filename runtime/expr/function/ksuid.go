package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
	"github.com/segmentio/ksuid"
)

type KSUID struct {
	sctx *super.Context
}

func (*KSUID) needsInput() {}

func (k *KSUID) Call(args ...vector.Any) vector.Any {
	if len(args) == 1 {
		n := args[0].Len()
		out := vector.NewBytesEmpty(n)
		for range n {
			out.Append(ksuid.New().Bytes())
		}
		return out
	}
	vec := vector.Under(args[1])
	switch vec.Type().ID() {
	case super.IDBytes:
		var errs []uint32
		out := vector.NewStringEmpty(vec.Len())
		for i := range vec.Len() {
			bytes := vector.BytesValue(vec, i)
			id, err := ksuid.FromBytes(bytes)
			if err != nil {
				errs = append(errs, i)
				continue
			}
			out.Append(id.String())
		}
		errVec := vector.NewWrappedError(k.sctx, "ksuid: invalid ksuid value", vector.Pick(vec, errs))
		return vector.Combine(out, errs, errVec)
	case super.IDString:
		var errs []uint32
		out := vector.NewBytesEmpty(vec.Len())
		for i := uint32(0); i < vec.Len(); i++ {
			s := vector.StringValue(vec, i)
			id, err := ksuid.Parse(s)
			if err != nil {
				errs = append(errs, i)
				continue
			}
			out.Append(id.Bytes())
		}
		errVec := vector.NewWrappedError(k.sctx, "ksuid: invalid ksuid value", vector.Pick(vec, errs))
		return vector.Combine(out, errs, errVec)
	case super.IDNull:
		return vec
	default:
		return vector.NewWrappedError(k.sctx, "ksuid: argument must a bytes or string type", vec)
	}
}
