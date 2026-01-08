package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
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
		c := vector.NewConst(super.Null, 0, bitvec.Zero)
		return vector.NewMap(mtyp, offsets, c, c, bitvec.Zero)
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
	key := m.build(vecs[:len(m.entries)])
	val := m.build(vecs[len(m.entries):])
	off := uint32(0)
	offsets := []uint32{off}
	for range vecs[0].Len() {
		off += uint32(len(m.entries))
		offsets = append(offsets, off)
	}
	mtyp := m.sctx.LookupTypeMap(key.Type(), val.Type())
	return vector.NewMap(mtyp, offsets, key, val, bitvec.Zero)
}

func (m *MapExpr) build(vecs []vector.Any) vector.Any {
	var typs []super.Type
	for _, vec := range vecs {
		typs = append(typs, vec.Type())
	}
	var b scode.Builder
	vb := vector.NewDynamicBuilder()
	for i := range vecs[0].Len() {
		for k, vec := range vecs {
			b.Truncate()
			vec.Serialize(&b, i)
			vb.Write(super.NewValue(typs[k], b.Bytes().Body()))
		}
	}
	out := vb.Build()
	if d, ok := out.(*vector.Dynamic); ok {
		utyp := m.sctx.LookupTypeUnion(super.UniqueTypes(typs))
		out = &vector.Union{Typ: utyp, Dynamic: d}
	}
	return out
}
