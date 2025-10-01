package cast

import (
	"github.com/brimdata/super"
	samexpr "github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

func To(sctx *super.Context, vec vector.Any, typ super.Type) vector.Any {
	vec = vector.Under(vec)
	var c caster
	id := typ.ID()
	if super.IsNumber(id) {
		c = func(vec vector.Any, index []uint32) (vector.Any, []uint32, string, bool) {
			return castToNumber(vec, typ, index)
		}
	} else {
		switch id {
		case super.IDBool:
			c = castToBool
		case super.IDString:
			c = castToString
		case super.IDBytes:
			c = castToBytes
		case super.IDIP:
			c = castToIP
		case super.IDNet:
			c = castToNet
		case super.IDType:
			c = func(vec vector.Any, index []uint32) (vector.Any, []uint32, string, bool) {
				return castToType(sctx, vec, index)
			}
		default:
			return errCastFailed(sctx, vec, typ, "")
		}
	}
	return assemble(sctx, vec, typ, c)
}

type caster func(vector.Any, []uint32) (vector.Any, []uint32, string, bool)

func assemble(sctx *super.Context, vec vector.Any, typ super.Type, fn caster) vector.Any {
	var out vector.Any
	var errs []uint32
	var errMsg string
	var ok bool
	switch vec := vec.(type) {
	case *vector.Const:
		return castConst(sctx, vec, typ)
	case *vector.View:
		out, errs, errMsg, ok = fn(vec.Any, vec.Index)
	case *vector.Dict:
		out, errs, errMsg, ok = fn(vec.Any, nil)
		if ok {
			if len(errs) > 0 {
				index, counts, nulls, nerrs := vec.RebuildDropTags(errs...)
				errs = nerrs
				out = vector.NewDict(out, index, counts, nulls)
			} else {
				out = vector.NewDict(out, vec.Index, vec.Counts, vec.Nulls)
			}
		}
	default:
		out, errs, errMsg, ok = fn(vec, nil)
	}
	if !ok {
		return errCastFailed(sctx, vec, typ, errMsg)
	}
	if len(errs) > 0 {
		return vector.Combine(out, errs, errCastFailed(sctx, vector.Pick(vec, errs), typ, errMsg))
	}
	return out
}

func castConst(sctx *super.Context, vec *vector.Const, typ super.Type) vector.Any {
	if vec.Type().ID() == super.IDNull {
		return vector.NewConst(super.NewValue(typ, nil), vec.Len(), bitvec.Zero)
	}
	val := samexpr.LookupPrimitiveCaster(sctx, typ).Eval(vec.Value())
	if val.IsError() {
		if !vec.Nulls.IsZero() {
			var trueCount uint32
			index := make([]uint32, vec.Nulls.Len())
			for i := range vec.Len() {
				if vec.Nulls.IsSet(i) {
					index[i] = 1
					trueCount++
				}
			}
			err := errCastFailed(sctx, vector.NewConst(vec.Value(), vec.Len()-trueCount, bitvec.Zero), typ, "")
			nulls := vector.NewConst(super.NewValue(typ, nil), trueCount, bitvec.Zero)
			return vector.NewDynamic(index, []vector.Any{err, nulls})
		}
		return errCastFailed(sctx, vec, typ, "")
	}
	return vector.NewConst(val, vec.Len(), vec.Nulls)
}

func errCastFailed(sctx *super.Context, vec vector.Any, typ super.Type, msgSuffix string) vector.Any {
	msg := "cannot cast to " + sup.FormatType(typ)
	if msgSuffix != "" {
		msg = msg + ": " + msgSuffix
	}
	return vector.NewWrappedError(sctx, msg, vec)
}

func lengthOf(vec vector.Any, index []uint32) uint32 {
	if index != nil {
		return uint32(len(index))
	}
	return vec.Len()
}
