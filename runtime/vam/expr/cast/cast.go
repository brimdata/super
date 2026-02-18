package cast

import (
	"github.com/brimdata/super"
	samexpr "github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
)

func To(sctx *super.Context, vec vector.Any, typ super.Type) vector.Any {
	if named, ok := typ.(*super.TypeNamed); ok {
		return castNamed(sctx, vec, named)
	}
	vec = vector.Under(vec)
	switch vec.Kind() {
	case vector.KindNull:
		return errCastFailed(sctx, vec, typ, "")
	case vector.KindError:
		return vec
	}
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
				index, counts, nerrs := vec.RebuildDropTags(errs...)
				errs = nerrs
				out = vector.NewDict(out, index, counts)
			} else {
				out = vector.NewDict(out, vec.Index, vec.Counts)
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
	val := samexpr.LookupPrimitiveCaster(sctx, typ).Eval(vec.Value())
	if val.IsError() {
		return errCastFailed(sctx, vec, typ, "")
	}
	return vector.NewConst(val, vec.Len())
}

func castNamed(sctx *super.Context, vec vector.Any, named *super.TypeNamed) vector.Any {
	return vector.Apply(false, func(vecs ...vector.Any) vector.Any {
		vec := vecs[0]
		if vec.Kind() == vector.KindError {
			return vec
		}
		return vector.NewNamed(named, vec)
	}, To(sctx, vec, named.Type))
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
