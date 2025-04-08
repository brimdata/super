package op

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
)

type Over struct {
	sctx   *super.Context
	parent vector.Puller
	exprs  []expr.Evaluator

	vecs []vector.Any
	idx  uint32
}

func NewOver(sctx *super.Context, parent vector.Puller, exprs []expr.Evaluator) *Over {
	return &Over{
		sctx:   sctx,
		parent: parent,
		exprs:  exprs,
	}
}

func (o *Over) Pull(done bool) (vector.Any, error) {
	if done {
		o.vecs = nil
		return o.parent.Pull(true)
	}
	for {
		if len(o.vecs) == 0 || o.idx >= o.vecs[0].Len() {
			vec, err := o.parent.Pull(done)
			if vec == nil || err != nil {
				return nil, err
			}
			o.vecs = o.vecs[:0]
			for _, e := range o.exprs {
				vec2 := e.Eval(vec)
				vec2 = vector.Apply(true, func(vecs ...vector.Any) vector.Any { return vecs[0] }, vec2)
				o.vecs = append(o.vecs, vec2)
			}
			o.idx = 0
		}
		var out vector.Any
		if len(o.vecs) == 1 {
			out = o.flatten(o.vecs[0], o.idx)
		} else {
			var tags []uint32
			var vecs []vector.Any
			for i, vec := range o.vecs {
				vec := o.flatten(vec, o.idx)
				for range vec.Len() {
					tags = append(tags, uint32(i))
				}
				vecs = append(vecs, vec)
			}
			out = vector.NewDynamic(tags, vecs)
		}
		o.idx++
		if out != nil {
			return out, nil
		}

	}
}

func (o *Over) flatten(vec vector.Any, slot uint32) vector.Any {
	switch vec := vector.Under(vec).(type) {
	case *vector.Dynamic:
		return o.flatten(vec.Values[vec.Tags[slot]], vec.TagMap.Forward[slot])
	case *vector.View:
		return o.flatten(vec.Any, vec.Index[slot])
	case *vector.Array:
		return flattenArrayOrSet(vec.Values, vec.Offsets, slot)
	case *vector.Set:
		return flattenArrayOrSet(vec.Values, vec.Offsets, slot)
	case *vector.Map:
		panic("unimplemented")
	case *vector.Record:
		if len(vec.Fields) == 0 || vec.Nulls.Value(slot) {
			return nil
		}
		keyType := o.sctx.LookupTypeArray(super.TypeString)
		keyOffsets := []uint32{0, 1}
		var tags []uint32
		var vecs []vector.Any
		for i, f := range super.TypeRecordOf(vec.Type()).Fields {
			tags = append(tags, uint32(i))
			typ := o.sctx.MustLookupTypeRecord([]super.Field{
				{Name: "key", Type: keyType},
				{Name: "value", Type: f.Type},
			})
			keyVec := vector.NewArray(keyType, keyOffsets, vector.NewConst(super.NewString(f.Name), 1, nil), nil)
			valVec := vector.NewView(vec.Fields[i], []uint32{slot})
			vecs = append(vecs, vector.NewRecord(typ, []vector.Any{keyVec, valVec}, keyVec.Len(), nil))
		}
		return vector.NewDynamic(tags, vecs)
	}
	return vector.NewView(vec, []uint32{slot})
}

func flattenArrayOrSet(vec vector.Any, offsets []uint32, slot uint32) vector.Any {
	var index []uint32
	for i := offsets[slot]; i < offsets[slot+1]; i++ {
		index = append(index, i)
	}
	if len(index) == 0 {
		return nil
	}
	return vector.NewView(vector.Deunion(vec), index)
}

type Scope struct {
	over    *Over
	sendEOS bool
}

func (o *Over) NewScope() *Scope {
	return &Scope{o, false}
}

func (s *Scope) Pull(done bool) (vector.Any, error) {
	if s.sendEOS || done {
		s.sendEOS = false
		return nil, nil
	}
	vec, err := s.over.Pull(false)
	s.sendEOS = vec != nil
	return vec, err
}

type ScopeExit struct {
	over     *Over
	parent   vector.Puller
	firstEOS bool
}

func (o *Over) NewScopeExit(parent vector.Puller) *ScopeExit {
	return &ScopeExit{o, parent, false}
}

func (s *ScopeExit) Pull(done bool) (vector.Any, error) {
	if done {
		vec, err := s.parent.Pull(true)
		if vec == nil || err != nil {
			return vec, err
		}
		return s.over.Pull(true)
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
