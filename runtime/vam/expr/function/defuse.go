package function

import (
	"fmt"

	"github.com/brimdata/super"
	samfunc "github.com/brimdata/super/runtime/sam/expr/function"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
)

type defuse struct {
	sctx     *super.Context
	downcast *downcast
	// This is used only for HasFusion func.
	samdefuse *samfunc.Defuse
}

func newDefuse(sctx *super.Context) *defuse {
	d := &defuse{
		sctx:      sctx,
		downcast:  &downcast{sctx: sctx},
		samdefuse: samfunc.NewDefuse(sctx),
	}
	d.downcast.defuser = d
	return d
}

func (d *defuse) Call(args ...vector.Any) vector.Any {
	vec, _ := d.eval(args[0])
	return vec
}

func (d *defuse) eval(in vector.Any) (vector.Any, bool) {
	if !d.samdefuse.HasFusion(in.Type()) {
		return in, true
	}
	switch in.Kind() {
	case vector.KindRecord:
		return d.defuseRecord(in)
	case vector.KindArray:
		return d.defuseArray(in)
	case vector.KindSet:
		return d.defuseSet(in)
	case vector.KindMap:
		return d.defuseMap(in)
	case vector.KindUnion:
		// XXX This should use vector.Apply but right now Apply defuses fusion
		// values and we do not want this here.
		dynamic := vector.Deunion(in).(*vector.Dynamic)
		var vecs []vector.Any
		ok := true
		for _, vec := range dynamic.Values {
			v, vok := d.eval(vec)
			ok = ok && vok
			vecs = append(vecs, v)
		}
		return vector.NewDynamic(dynamic.Tags, vecs), ok
	case vector.KindFusion:
		fusion := expr.PushContainerViewDown(in).(*vector.Fusion)
		val, ok := d.downcast.defuseWithErrors(fusion.Values, fusion.Subtypes.Types())
		fmt.Println("DEFUSE GOT", ok)
		vector.Println(val)
		return val, ok

	default:
		// primitives, named types, enums
		// BTW, named types are a barrier to defuse.
		return in, true
	}
}

func (d *defuse) defuseRecord(vec vector.Any) (vector.Any, bool) {
	rec := expr.PushContainerViewDown(vec).(*vector.Record)
	var vecs []vector.Any
	ok := true
	for _, vec := range rec.Fields {
		v, vok := d.eval(vec)
		ok = ok && vok
		vecs = append(vecs, v)
	}
	// Append length so this still works with empty records.
	vecs = append(vecs, vector.NewNull(rec.Len()))
	return vector.Apply(false, func(vecs ...vector.Any) vector.Any {
		n := vecs[len(vecs)-1].Len()
		vecs = vecs[:len(vecs)-1]
		var fields []super.Field
		for i, f := range rec.Typ.Fields {
			vec := vecs[i]
			if vec.Kind() == vector.KindNone {
				continue
			}
			fields = append(fields, super.NewField(f.Name, vec.Type()))
		}
		typ := d.sctx.MustLookupTypeRecord(fields)
		return vector.NewRecord(typ, vecs, n)
	}, vecs...), ok
}

func (d *defuse) defuseArray(in vector.Any) (vector.Any, bool) {
	array := expr.PushContainerViewDown(in).(*vector.Array)
	inner, ok := d.eval(array.Values)
	if !vector.IsDynamic(inner) {
		return vector.NewArray(d.sctx.LookupTypeArray(inner.Type()), array.Offsets, inner), ok
	}
	tags, inners, offsets := expr.SplitListByTypes(d.sctx, array.Offsets, inner)
	var vals []vector.Any
	for i, inner := range inners {
		typ := d.sctx.LookupTypeArray(inner.Type())
		vals = append(vals, vector.NewArray(typ, offsets[i], inner))
	}
	if len(vals) > 1 {
		return vector.NewDynamic(tags, vals), ok
	}
	return vals[0], ok
}

func (d *defuse) defuseSet(in vector.Any) (vector.Any, bool) {
	set := expr.PushContainerViewDown(in).(*vector.Set)
	inner, ok := d.eval(set.Values)
	if !vector.IsDynamic(inner) {
		return vector.NewSet(d.sctx.LookupTypeSet(inner.Type()), set.Offsets, inner), ok
	}
	tags, inners, offsets := expr.SplitListByTypes(d.sctx, set.Offsets, inner)
	var vals []vector.Any
	for i, inner := range inners {
		typ := d.sctx.LookupTypeSet(inner.Type())
		vals = append(vals, vector.NewSet(typ, offsets[i], inner))
	}
	if len(vals) > 1 {
		return vector.NewDynamic(tags, vals), ok
	}
	return vals[0], ok
}

func (d *defuse) defuseMap(in vector.Any) (vector.Any, bool) {
	vmap := expr.PushContainerViewDown(in).(*vector.Map)
	keys, ok := d.eval(vmap.Values)
	vals, vok := d.eval(vmap.Values)
	ok = ok && vok
	if !vector.IsDynamic(keys) && !vector.IsDynamic(vals) {
		typ := d.sctx.LookupTypeMap(keys.Type(), vals.Type())
		return vector.NewMap(typ, vmap.Offsets, keys, vals), ok
	}
	keySlotTypes := expr.SlotTypesInList(d.sctx, keys, vmap.Offsets)
	valSlotTypes := expr.SlotTypesInList(d.sctx, vals, vmap.Offsets)
	type mapType struct {
		key super.Type
		val super.Type
	}
	m := make(map[mapType][]uint32)
	for i := range vmap.Len() {
		mtyp := mapType{keySlotTypes[i], valSlotTypes[i]}
		m[mtyp] = append(m[mtyp], uint32(i))
	}
	tags := make([]uint32, len(vmap.Offsets)-1)
	var vecs []vector.Any
	for mtyp, index := range m {
		keys, offsets := expr.SubsetOfList(d.sctx, keys, vmap.Offsets, index, mtyp.key)
		vals, _ := expr.SubsetOfList(d.sctx, vals, vmap.Offsets, index, mtyp.val)
		for _, idx := range index {
			tags[idx] = uint32(len(vecs))
		}
		typ := d.sctx.LookupTypeMap(keys.Type(), vals.Type())
		vecs = append(vecs, vector.NewMap(typ, offsets, keys, vals))
	}
	if len(vecs) == 1 {
		return vecs[0], ok
	}
	return vector.NewDynamic(tags, vecs), ok
}
