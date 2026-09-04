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
	val := vector.Under(args[0])
	key := vector.Under(args[1])
	switch val.Kind() {
	case vector.KindType:
		if _, ok := val.Type().(*super.TypeOfType); ok {
			if key.Kind() != vector.KindString {
				return vector.NewWrappedError(h.sctx, "has function applied to type value with non-string key", key)
			}
			return h.hasTypeRecordField(val, key)
		}
		//XXX panic?
		return vector.NewFalse(val.Len())
	case vector.KindRecord:
		if typ, ok := val.Type().(*super.TypeRecord); ok {
			if key.Kind() != vector.KindString {
				return vector.NewWrappedError(h.sctx, "has function applied to record with non-string key", key)
			}
			return h.hasRecordField(typ, key)
		}
		//XXX panic?
		return vector.NewFalse(val.Len())
	case vector.KindMap:
		panic("TBD")
	default:
		return vector.NewWrappedError(h.sctx, "has function applied to invalid type", val)
	}
}

func (h *Has) hasRecordField(typ *super.TypeRecord, key vector.Any) vector.Any {
	n := key.Len()
	bits := bitvec.NewFalse(n)
	for slot := range n {
		if typ.HasField(vector.StringValue(key, slot)) {
			bits.Set(slot)
		}
	}
	return vector.NewBool(bits)
}

func (h *Has) hasTypeRecordField(val vector.Any, key vector.Any) vector.Any {
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
