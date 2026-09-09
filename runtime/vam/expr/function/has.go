package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type Has struct {
	sctx   *super.Context
	defuse *expr.Defuse
}

func newHas(sctx *super.Context) *Has {
	return &Has{sctx, expr.NewDefuse(sctx)}
}

func (*Has) NoDefuse() bool            { return false }
func (*Has) ApplyOpt() vector.ApplyOpt { return vector.ApplyRipUnions }

func (h *Has) Call(args ...vector.Any) vector.Any {
	val := h.defuse.Eval(vector.Under(args[0]))
	key := h.defuse.Eval(vector.Under(args[1]))
	return vector.Apply(vector.ApplyRipUnions, h.eval, val, key)
}

func (h *Has) eval(args ...vector.Any) vector.Any {
	val := vector.PushView(vector.Under(args[0]))
	key := vector.Under(args[1])
	switch val.Kind() {
	case vector.KindType:
		if key.Kind() != vector.KindString {
			return vector.NewWrappedError(h.sctx, "has function applied to type value with non-string key", key)
		}
		return h.hasTypeRecordField(val, key)
	case vector.KindRecord:
		if key.Kind() != vector.KindString {
			return vector.NewWrappedError(h.sctx, "has: applied to record with non-string key", key)
		}
		return h.hasRecordField(val, key)
	case vector.KindMap:
		return vector.NewWrappedError(h.sctx, "has: map types not yet supported", key)
	default:
		return vector.NewWrappedError(h.sctx, "has: invalid type", val)
	}
}

func (h *Has) hasRecordField(val, key vector.Any) vector.Any {
	switch val := val.(type) {
	case *vector.Const:
		return vector.NewConst(h.hasRecordField(val.Any, key), val.Len())
	case *vector.Dict:
		return vector.NewDict(h.hasRecordField(val.Any, key), val.Index, val.Counts)
	case *vector.View:
		return h.hasRecordField(vector.PushView(val), key)
	case *vector.Record:
		typ := val.Typ
		n := key.Len()
		bits := bitvec.NewFalse(n)
		for slot := range n {
			if k, ok := typ.IndexOfField(vector.StringValue(key, slot)); ok {
				if !vector.IsNone(val.Fields[k], slot) {
					bits.Set(slot)
				}
			}
		}
		return vector.NewBool(bits)
	}
	panic(val)
}

func (h *Has) hasTypeRecordField(val, key vector.Any) vector.Any {
	n := key.Len()
	bits := bitvec.NewFalse(n)
	switch val := val.(type) {
	case *vector.Const:
		return vector.NewConst(h.hasTypeRecordField(val.Any, key), n)
	case *vector.TypeValue:
		types := val.Types()
		for slot, t := range types {
			//XXX above returns error for non-record, this returns false
			if valType, ok := super.TypeUnder(t).(*super.TypeRecord); ok {
				if valType.HasField(vector.StringValue(key, uint32(slot))) {
					bits.Set(uint32(slot))
				}
			}
		}
	case *vector.View:
		return h.hasTypeRecordField(vector.PushView(val), key)
	}
	return vector.NewBool(bits)
}
