package agg

import (
	"github.com/brimdata/super"
	samagg "github.com/brimdata/super/runtime/sam/expr/agg"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector"
)

type union struct {
	samunion *samagg.Union
}

func newUnion() *union {
	return &union{samunion: samagg.NewUnion()}
}

func (u *union) Consume(vec vector.Any) {
	switch vec := vec.(type) {
	case *vector.Const:
		val := vec.Value()
		if val.IsNull() {
			return
		}
		u.samunion.Update(vec.Type(), val.Bytes())
	case *vector.Dict:
		u.Consume(vec.Any)
	case *vector.Error: // ignore
	default:
		nulls := vector.NullsOf(vec)
		typ := vec.Type()
		var b scode.Builder
		for i := range vec.Len() {
			if nulls.IsSet(i) {
				continue
			}
			b.Truncate()
			vec.Serialize(&b, i)
			u.samunion.Update(typ, b.Bytes().Body())
		}
	}
}

func (u *union) Result(sctx *super.Context) super.Value {
	return u.samunion.Result(sctx)
}

func (u *union) ConsumeAsPartial(partial vector.Any) {
	if c, ok := partial.(*vector.Const); ok && c.Value().IsNull() {
		return
	}
	n := partial.Len()
	var index []uint32
	if view, ok := partial.(*vector.View); ok {
		index = view.Index
		partial = view.Any
	}
	set, ok := partial.(*vector.Set)
	if !ok {
		panic("union: partial not a set type")
	}
	inner := set.Values
	typ := inner.Type()
	union, _ := typ.(*super.TypeUnion)
	var b scode.Builder
	for idx := range n {
		i := idx
		if index != nil {
			idx = index[i]
		}
		for k := set.Offsets[idx]; k < set.Offsets[idx+1]; k++ {
			b.Truncate()
			inner.Serialize(&b, k)
			bytes := b.Bytes().Body()
			if union != nil {
				typ, bytes = union.Untag(bytes)
			}
			u.samunion.Update(typ, bytes)
		}
	}
}

func (u *union) ResultAsPartial(sctx *super.Context) super.Value {
	return u.samunion.ResultAsPartial(sctx)
}
