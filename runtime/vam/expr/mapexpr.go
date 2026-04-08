package expr

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type Entry struct {
	Key Evaluator
	Val Evaluator
}

type MapExpr struct {
	sctx    *super.Context
	entries []Entry
}

func NewMapExpr(sctx *super.Context, entries []Entry) *MapExpr {
	return &MapExpr{
		sctx:    sctx,
		entries: entries,
	}
}

func (m *MapExpr) Eval(this vector.Any) vector.Any {
	if len(m.entries) == 0 {
		mtyp := m.sctx.LookupTypeMap(super.TypeNull, super.TypeNull)
		offsets := make([]uint32, this.Len()+1)
		c := vector.NewNull(0)
		return vector.NewMap(mtyp, offsets, c, c)
	}
	var vecs []vector.Any
	for _, entry := range m.entries {
		vecs = append(vecs, entry.Key.Eval(this))
	}
	for _, entry := range m.entries {
		vecs = append(vecs, entry.Val.Eval(this))
	}
	return vector.Apply(false, m.eval, vecs...)
}

func (m *MapExpr) eval(vecs ...vector.Any) vector.Any {
	n := len(m.entries)
	key := m.build(m.sctx, vecs[:n])
	val := m.build(m.sctx, vecs[n:])
	offsets := buildStaticOffsets(uint32(n), vecs[0].Len())
	mtyp := m.sctx.LookupTypeMap(key.Type(), val.Type())
	return vector.NewMap(mtyp, offsets, key, val)
}

func (m *MapExpr) build(sctx *super.Context, vecs []vector.Any) vector.Any {
	if len(vecs) == 1 {
		return vecs[0]
	}
	repeat := make([]uint32, len(vecs))
	for i := range uint32(len(vecs)) {
		repeat[i] = i
	}
	tags := slices.Repeat(repeat, int(vecs[0].Len()))
	d := vector.FlattenUnions(vector.NewDynamic(tags, vecs))
	out := vector.MergeSameTypesInDynamic(sctx, d)
	if d, ok := out.(*vector.Dynamic); ok {
		out = vector.NewUnionFromDynamic(m.sctx, d)
	}
	return out
}
