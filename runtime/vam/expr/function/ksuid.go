package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
	"github.com/segmentio/ksuid"
)

type KSUID struct {
	sctx *super.Context
}

func (*KSUID) needsInput() {}

func (k *KSUID) Call(args ...vector.Any) vector.Any {
	if len(args) == 1 {
		n := args[0].Len()
		out := vector.NewBytesEmpty(n, bitvec.Zero)
		for range n {
			out.Append(ksuid.New().Bytes())
		}
		return out
	}
	vec := vector.Under(args[1])
	switch vec.Type().ID() {
	case super.IDBytes:
		var errs []uint32
		var nulls bitvec.Bits
		out := vector.NewStringEmpty(vec.Len(), bitvec.Zero)
		for i := range vec.Len() {
			bytes, null := vector.BytesValue(vec, i)
			if null {
				if nulls.IsZero() {
					nulls = bitvec.NewFalse(vec.Len())
				}
				nulls.Set(i)
				out.Append("")
				continue
			}
			id, err := ksuid.FromBytes(bytes)
			if err != nil {
				errs = append(errs, i)
				continue
			}
			out.Append(id.String())
		}
		if !nulls.IsZero() {
			nulls.Shorten(out.Len())
			out.Nulls = nulls
		}
		errVec := vector.NewWrappedError(k.sctx, "ksuid: invalid ksuid value", vector.Pick(vec, errs))
		return vector.Combine(out, errs, errVec)
	case super.IDString:
		var errs []uint32
		var nulls bitvec.Bits
		out := vector.NewBytesEmpty(vec.Len(), bitvec.Zero)
		for i := uint32(0); i < vec.Len(); i++ {
			s, null := vector.StringValue(vec, i)
			if null {
				if nulls.IsZero() {
					nulls = bitvec.NewFalse(vec.Len())
				}
				nulls.Set(i)
				out.Append(nil)
				continue
			}
			id, err := ksuid.Parse(s)
			if err != nil {
				errs = append(errs, i)
				continue
			}
			out.Append(id.Bytes())
		}
		if !nulls.IsZero() {
			nulls.Shorten(out.Len())
			out.Nulls = nulls
		}
		errVec := vector.NewWrappedError(k.sctx, "ksuid: invalid ksuid value", vector.Pick(vec, errs))
		return vector.Combine(out, errs, errVec)
	default:
		return vector.NewWrappedError(k.sctx, "ksuid: argument must a bytes or string type", vec)
	}
}
