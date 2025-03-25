package cast

import (
	"github.com/brimdata/super"
	samexpr "github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zson"
)

func To(zctx *super.Context, vec vector.Any, typ super.Type) vector.Any {
	vec = vector.Under(vec)
	var c caster
	id := typ.ID()
	if super.IsNumber(id) {
		c = func(vec vector.Any, index []uint32) (vector.Any, []uint32, bool) {
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
			c = func(vec vector.Any, index []uint32) (vector.Any, []uint32, bool) {
				return castToType(zctx, vec, index)
			}
		default:
			return errCastFailed(zctx, vec, typ)
		}
	}
	return assemble(zctx, vec, typ, c)
}

type caster func(vector.Any, []uint32) (vector.Any, []uint32, bool)

func assemble(zctx *super.Context, vec vector.Any, typ super.Type, fn caster) vector.Any {
	var out vector.Any
	var errs []uint32
	var ok bool
	switch vec := vec.(type) {
	case *vector.Const:
		return castConst(zctx, vec, typ)
	case *vector.View:
		out, errs, ok = fn(vec.Any, vec.Index)
	case *vector.Dict:
		out, errs, ok = fn(vec.Any, nil)
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
		out, errs, ok = fn(vec, nil)
	}
	if !ok {
		return errCastFailed(zctx, vec, typ)
	}
	if len(errs) > 0 {
		return vector.Combine(out, errs, errCastFailed(zctx, vector.NewView(vec, errs), typ))
	}
	return out
}

func castConst(zctx *super.Context, vec *vector.Const, typ super.Type) vector.Any {
	if vec.Type().ID() == super.IDNull {
		return vector.NewConst(super.NewValue(typ, nil), vec.Len(), nil)
	}
	val := samexpr.LookupPrimitiveCaster(zctx, typ).Eval(samexpr.NewContext(), vec.Value())
	if val.IsError() {
		if vec.Nulls != nil {
			var trueCount uint32
			index := make([]uint32, vec.Nulls.Len())
			for i := range vec.Len() {
				if vec.Nulls.Value(i) {
					index[i] = 1
					trueCount++
				}
			}
			err := errCastFailed(zctx, vector.NewConst(vec.Value(), vec.Len()-trueCount, nil), typ)
			nulls := vector.NewConst(super.NewValue(typ, nil), trueCount, nil)
			return vector.NewDynamic(index, []vector.Any{err, nulls})
		}
		return errCastFailed(zctx, vec, typ)
	}
	return vector.NewConst(val, vec.Len(), vec.Nulls)
}

func errCastFailed(zctx *super.Context, vec vector.Any, typ super.Type) vector.Any {
	return vector.NewWrappedError(zctx, "cannot cast to "+zson.FormatType(typ), vec)
}

func lengthOf(vec vector.Any, index []uint32) uint32 {
	if index != nil {
		return uint32(len(index))
	}
	return vec.Len()
}
