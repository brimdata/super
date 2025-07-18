package op

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type Unnest struct {
	sctx   *super.Context
	parent vector.Puller
	expr   expr.Evaluator

	vecs []vector.Any
	idx  uint32
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
		u.vecs = nil
		return u.parent.Pull(true)
	}
	for {
		if len(u.vecs) == 0 || u.idx >= u.vecs[0].Len() {
			vec, err := u.parent.Pull(done)
			if vec == nil || err != nil {
				return nil, err
			}
			u.vecs = u.vecs[:0]
			vec2 := u.expr.Eval(vec)
			vec2 = vector.Apply(true, func(vecs ...vector.Any) vector.Any { return vecs[0] }, vec2)
			u.vecs = append(u.vecs, vec2)
			u.idx = 0
		}
		var out vector.Any
		if len(u.vecs) == 1 {
			out = u.flatten(u.vecs[0], u.idx)
		} else {
			var tags []uint32
			var vecs []vector.Any
			for i, vec := range u.vecs {
				vec := u.flatten(vec, u.idx)
				for range vec.Len() {
					tags = append(tags, uint32(i))
				}
				vecs = append(vecs, vec)
			}
			out = vector.NewDynamic(tags, vecs)
		}
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
		if vec.Nulls.IsSet(slot) {
			return nil
		}
		if len(vec.Fields) != 2 {
			return vector.NewWrappedError(u.sctx, "unnest: encountered record without two columns", vec)
		}
		if super.InnerType(vec.Fields[1].Type()) == nil {
			return vector.NewWrappedError(u.sctx, "unnest: encountered record without an array column", vec)
		}
		right := u.flatten(vec.Fields[1], slot)
		lindex := make([]uint32, right.Len())
		left := vector.NewView(vector.Pick(vec.Fields[0], []uint32{slot}), lindex)
		return vector.Apply(true, func(vecs ...vector.Any) vector.Any {
			fields := slices.Clone(vec.Typ.Fields)
			fields[1].Type = vecs[1].Type()
			typ := u.sctx.MustLookupTypeRecord(fields)
			return vector.NewRecord(typ, vecs, vecs[0].Len(), bitvec.Zero)
		}, left, right)
	default:
		return vector.NewWrappedError(u.sctx, "unnest: encountered non-array value", vec)
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

type Scope struct {
	unnest  *Unnest
	sendEOS bool
}

func (u *Unnest) NewScope() *Scope {
	return &Scope{u, false}
}

func (s *Scope) Pull(done bool) (vector.Any, error) {
	if s.sendEOS || done {
		s.sendEOS = false
		return nil, nil
	}
	vec, err := s.unnest.Pull(false)
	s.sendEOS = vec != nil
	return vec, err
}

type ScopeExit struct {
	unnest   *Unnest
	parent   vector.Puller
	firstEOS bool
}

func (u *Unnest) NewScopeExit(parent vector.Puller) *ScopeExit {
	return &ScopeExit{u, parent, false}
}

func (s *ScopeExit) Pull(done bool) (vector.Any, error) {
	if done {
		vec, err := s.parent.Pull(true)
		if vec == nil || err != nil {
			return vec, err
		}
		return s.unnest.Pull(true)
	}
	for {
		vec, err := s.parent.Pull(false)
		if err != nil {
			return nil, err
		}
		if vec != nil {
			s.firstEOS = false
			return vec, nil
		}
		if s.firstEOS {
			return nil, nil
		}
		s.firstEOS = true
	}
}
