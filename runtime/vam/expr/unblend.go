package expr

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

func Unblend(sctx *super.Context, vec vector.Any) vector.Any {
	switch vec.Kind() {
	case vector.KindRecord:
		return unblendRecord(sctx, vec)
	case vector.KindArray:
		array := pushContainerViewDown(vec).(*vector.Array)
		tags, inners, offsets := unblendArrayOrSet(sctx, array.Offsets, array.Values)
		var vals []vector.Any
		for i, inner := range inners {
			typ := sctx.LookupTypeArray(inner.Type())
			vals = append(vals, vector.NewArray(typ, offsets[i], inner))
		}
		if len(vals) > 1 {
			return vector.NewDynamic(tags, vals)
		}
		return vals[0]
	case vector.KindSet:
		set := pushContainerViewDown(vec).(*vector.Set)
		tags, inners, offsets := unblendArrayOrSet(sctx, set.Offsets, set.Values)
		var vals []vector.Any
		for i, inner := range inners {
			typ := sctx.LookupTypeSet(inner.Type())
			vals = append(vals, vector.NewSet(typ, offsets[i], inner))
		}
		if len(vals) > 1 {
			return vector.NewDynamic(tags, vals)
		}
		return vals[0]
	case vector.KindMap:
		return unblendMap(sctx, pushContainerViewDown(vec).(*vector.Map))
	case vector.KindUnion:
		out := vector.Apply(true, func(vecs ...vector.Any) vector.Any {
			return Unblend(sctx, vecs[0])
		}, vec)
		if dynamic, ok := out.(*vector.Dynamic); ok {
			idx := -1
			// If dynamic only has a single Value of len > 0 return the value.
			for i, val := range dynamic.Values {
				if val != nil && val.Len() > 0 {
					if idx != -1 {
						return out
					}
					idx = i
				}
			}
			return dynamic.Values[idx]
		}
		return out
	}
	return vec
}

func unblendRecord(sctx *super.Context, in vector.Any) vector.Any {
	vec := in
	var index []uint32
	if view, ok := vec.(*vector.View); ok {
		index = view.Index
		vec = view.Any
	}
	rec := vec.(*vector.Record)
	if len(rec.Fields) == 0 {
		return in
	}
	fields := slices.Clone(rec.Fields)
	if index != nil {
		for i, field := range fields {
			fields[i] = vector.Pick(field, index)
		}
	}
	for i, field := range fields {
		fields[i] = Unblend(sctx, field)
	}
	return vector.Apply(false, func(vecs ...vector.Any) vector.Any {
		var fields []super.Field
		var fvecs []vector.Any
		for i, vec := range vecs {
			if _, ok := vec.(*vector.None); ok {
				continue
			}
			fvecs = append(fvecs, vec)
			fields = append(fields, super.NewField(rec.Typ.Fields[i].Name, vec.Type()))
		}
		rtyp := sctx.MustLookupTypeRecord(fields)
		return vector.NewRecord(rtyp, fvecs, vecs[0].Len())
	}, fields...)
}

func unblendArrayOrSet(sctx *super.Context, offsets []uint32, elements vector.Any) ([]uint32, []vector.Any, [][]uint32) {
	elements = Unblend(sctx, elements)
	dynamic, ok := elements.(*vector.Dynamic)
	if !ok {
		return nil, []vector.Any{elements}, [][]uint32{offsets}
	}
	union := vector.NewUnionFromDynamic(sctx, dynamic)
	slotTypes := typesOfSlotsInList(sctx, union, offsets)
	// Accumulate unique array types.
	m := make(map[super.Type][]uint32)
	for i, typ := range slotTypes {
		m[typ] = append(m[typ], uint32(i))
	}
	dtags := make([]uint32, len(offsets)-1)
	var inners []vector.Any
	var offs [][]uint32
	for typ, index := range m {
		inner, off := subsetOfList(sctx, union, offsets, index, typ)
		for _, idx := range index {
			dtags[idx] = uint32(len(inners))
		}
		inners = append(inners, inner)
		offs = append(offs, off)
	}
	return dtags, inners, offs
}

func unblendMap(sctx *super.Context, vmap *vector.Map) vector.Any {
	keys := Unblend(sctx, vmap.Keys)
	if dynamic, ok := keys.(*vector.Dynamic); ok {
		keys = vector.NewUnionFromDynamic(sctx, dynamic)
	}
	vals := Unblend(sctx, vmap.Values)
	if dynamic, ok := vals.(*vector.Dynamic); ok {
		vals = vector.NewUnionFromDynamic(sctx, dynamic)
	}
	if keys.Kind() != vector.KindUnion && vals.Kind() != vector.KindUnion {
		mtyp := sctx.LookupTypeMap(keys.Type(), vals.Type())
		return vector.NewMap(mtyp, vmap.Offsets, keys, vals)
	}
	keySlotTypes := typesOfSlotsInList(sctx, keys, vmap.Offsets)
	valSlotTypes := typesOfSlotsInList(sctx, vals, vmap.Offsets)
	type mapType struct {
		key super.Type
		val super.Type
	}
	// Accumulate unique map types.
	m := make(map[mapType][]uint32)
	for i := range vmap.Len() {
		mtyp := mapType{keySlotTypes[i], valSlotTypes[i]}
		m[mtyp] = append(m[mtyp], uint32(i))
	}
	dtags := make([]uint32, len(vmap.Offsets)-1)
	var vecs []vector.Any
	for mtyp, index := range m {
		keys, offsets := subsetOfList(sctx, keys, vmap.Offsets, index, mtyp.key)
		vals, _ := subsetOfList(sctx, vals, vmap.Offsets, index, mtyp.val)
		for _, idx := range index {
			dtags[idx] = uint32(len(vecs))
		}
		typ := sctx.LookupTypeMap(keys.Type(), vals.Type())
		vecs = append(vecs, vector.NewMap(typ, offsets, keys, vals))
	}
	if len(vecs) == 1 {
		return vecs[0]
	}
	return vector.NewDynamic(dtags, vecs)
}

func subsetOfList(sctx *super.Context, elements vector.Any, parentOffsets, index []uint32, typ super.Type) (vector.Any, []uint32) {
	if typ == super.TypeNull {
		nulls := vector.NewNull(uint32(len(index)))
		offsets := make([]uint32, len(index)+1)
		return nulls, offsets
	}
	var allVals []vector.Any
	union, ok := elements.(*vector.Union)
	if ok {
		allVals = union.Values
	} else {
		allVals = []vector.Any{elements}
	}
	var subTypes []super.Type
	utyp, ok := typ.(*super.TypeUnion)
	if ok {
		subTypes = slices.Clone(utyp.Types)
	} else {
		subTypes = append(subTypes, typ)
	}
	subVals := make([]vector.Any, len(subTypes))
	// map parent union tags to subset union tags
	tagMap := make([]uint32, len(allVals))
	for i, typ := range subTypes {
		idx := slices.IndexFunc(allVals, func(vec vector.Any) bool {
			return vec.Type() == typ
		})
		tagMap[idx] = uint32(i)
		subVals[i] = allVals[idx]
	}
	// Generate:
	// - offsets for new array
	// - indexes to create view on values
	// - tags for union (if applicable)
	var forwardTags []uint32
	if union != nil {
		forwardTags = union.ForwardTagMap()
	}
	var tags []uint32
	indexes := make([][]uint32, len(subTypes))
	suboffsets := []uint32{0}
	for _, idx := range index {
		start, end := parentOffsets[idx], parentOffsets[idx+1]
		if union != nil {
			for i, origTag := range union.Tags[start:end] {
				tag := tagMap[origTag]
				tags = append(tags, tag)
				indexes[tag] = append(indexes[tag], forwardTags[start+uint32(i)])
			}
		} else {
			for i := start; i < end; i++ {
				indexes[0] = append(indexes[0], i)
			}
		}
		suboffsets = append(suboffsets, uint32(len(tags)))
	}
	for i, val := range subVals {
		subVals[i] = vector.Pick(val, indexes[i])
	}
	var inner vector.Any
	if len(subVals) > 1 {
		union, ok := sctx.LookupTypeUnion(subTypes)
		if !ok {
			panic(subTypes)
		}
		inner = vector.NewUnion(union, tags, subVals)
	} else {
		inner = subVals[0]
	}
	return inner, suboffsets
}

func typesOfSlotsInList(sctx *super.Context, inner vector.Any, offsets []uint32) []super.Type {
	union, _ := inner.(*vector.Union)
	var alltypes []super.Type
	if union != nil {
		for _, val := range union.Values {
			alltypes = append(alltypes, val.Type())
		}
	} else {
		alltypes = []super.Type{inner.Type()}
	}
	n := uint32(len(offsets) - 1)
	slotTypes := make([]super.Type, n)
	for i := range n {
		if union != nil {
			slotTypes[i] = typeOfRange(sctx, union, alltypes, offsets[i], offsets[i+1])
		} else {
			slotTypes[i] = alltypes[0]
		}
	}
	return slotTypes
}

func typeOfRange(sctx *super.Context, union *vector.Union, alltypes []super.Type, start, end uint32) super.Type {
	tags := slices.Clone(union.Tags[start:end])
	slices.Sort(tags)
	uniq := slices.Compact(tags)
	if len(uniq) == 0 {
		return super.TypeNull
	}
	if len(uniq) == 1 {
		return alltypes[uniq[0]]
	}
	if len(uniq) == len(alltypes) {
		return union.Typ
	}
	var types []super.Type
	for _, tag := range uniq {
		types = append(types, alltypes[tag])
	}
	out, ok := sctx.LookupTypeUnion(types)
	if !ok {
		panic(types)
	}
	return out
}

func pushContainerViewDown(val vector.Any) vector.Any {
	view, ok := val.(*vector.View)
	if !ok {
		return val
	}
	switch val := view.Any.(type) {
	case *vector.Array:
		inner, offsets := pickList(val.Values, view.Index, val.Offsets)
		return vector.NewArray(val.Typ, offsets, inner)
	case *vector.Set:
		inner, offsets := pickList(val.Values, view.Index, val.Offsets)
		return vector.NewSet(val.Typ, offsets, inner)
	case *vector.Map:
		keys, offsets := pickList(val.Keys, view.Index, val.Offsets)
		values, _ := pickList(val.Values, view.Index, val.Offsets)
		return vector.NewMap(val.Typ, offsets, keys, values)
	default:
		panic(val)
	}
}

func pickList(inner vector.Any, index, offsets []uint32) (vector.Any, []uint32) {
	newOffsets := []uint32{0}
	var innerIndex []uint32
	for _, idx := range index {
		start, end := offsets[idx], offsets[idx+1]
		for ; start < end; start++ {
			innerIndex = append(innerIndex, start)
		}
		newOffsets = append(newOffsets, uint32(len(innerIndex)))
	}
	return vector.Pick(inner, innerIndex), newOffsets
}
