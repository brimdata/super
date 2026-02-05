package op

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
)

type Unnest struct {
	sctx   *super.Context
	parent vector.Puller
	expr   expr.Evaluator

	vec vector.Any
	idx uint32
}

func NewUnnest(sctx *super.Context, parent vector.Puller, expr expr.Evaluator) *Unnest {
	return &Unnest{
		sctx:   sctx,
		parent: parent,
		expr:   expr,
	}
}

func (u *Unnest) Pull(done bool) (vector.Any, error) {
	if done {
		u.vec = nil
		return u.parent.Pull(true)
	}
	for {
		if u.vec == nil || u.idx >= u.vec.Len() {
			vec, err := u.parent.Pull(done)
			if vec == nil || err != nil {
				return nil, err
			}
			u.vec = u.expr.Eval(vec)
			u.idx = 0
		}
		out := u.flatten(u.vec, u.idx)
		u.idx++
		if out != nil {
			return out, nil
		}

	}
}

func (u *Unnest) flatten(vec vector.Any, slot uint32) vector.Any {
	switch vec := vector.Under(vec).(type) {
	case *vector.Dynamic:
		return u.flatten(vec.Values[vec.Tags[slot]], vec.ForwardTagMap()[slot])
	case *vector.View:
		return u.flatten(vec.Any, vec.Index[slot])
	case *vector.Array:
		return flattenArrayOrSet(vec.Values, vec.Offsets, slot)
	case *vector.Set:
		return flattenArrayOrSet(vec.Values, vec.Offsets, slot)
	case *vector.Record:
		fields := vec.Fields(u.sctx)
		if len(fields) != 2 {
			return vector.NewWrappedError(u.sctx, "unnest: encountered record without two fields", vec)
		}
		if super.InnerType(fields[1].Type()) == nil {
			return vector.NewWrappedError(u.sctx, "unnest: encountered record without an array/set type for second field", vec)
		}
		right := u.flatten(fields[1], slot)
		lindex := make([]uint32, right.Len())
		left := vector.NewView(vector.Pick(fields[0], []uint32{slot}), lindex)
		return vector.Apply(true, func(vecs ...vector.Any) vector.Any {
			fields := slices.Clone(vec.Typ.Fields)
			fields[1].Type = vecs[1].Type()
			typ := u.sctx.MustLookupTypeRecord(fields)
			return vector.NewRecord(typ, vecs, vecs[0].Len())
		}, left, right)
	case *vector.Union:
		return u.flatten(vec.Dynamic, slot)
	default:
		if vec.Kind() == vector.KindNull {
			return nil
		}
		slotVec := vector.Pick(vec, []uint32{slot})
		return vector.NewWrappedError(u.sctx, "unnest: encountered non-array value", slotVec)
	}
}

func flattenArrayOrSet(vec vector.Any, offsets []uint32, slot uint32) vector.Any {
	var index []uint32
	for i := offsets[slot]; i < offsets[slot+1]; i++ {
		index = append(index, i)
	}
	if len(index) == 0 {
		return nil
	}
	return vector.Pick(vector.Deunion(vec), index)
}
