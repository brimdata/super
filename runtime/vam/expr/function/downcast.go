package function

import (
	"math"
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
)

type downcast struct {
	sctx *super.Context
}

func (d *downcast) Call(vecs ...vector.Any) vector.Any {
	from, to := vecs[0], vecs[1]
	if to.Kind() != vector.KindType {
		return vector.NewWrappedError(d.sctx, "downcast: type argument not a type", to)
	}
	switch to := to.(type) {
	case *vector.View:
		allTypes := to.Any.(*vector.TypeValue).Types()
		types := make([]super.Type, len(to.Index))
		for i, slot := range to.Index {
			types[i] = allTypes[slot]
		}
		return d.call(from, types)
	case *vector.Dict:
		dictTypes := to.Any.(*vector.TypeValue).Types()
		types := make([]super.Type, len(to.Index))
		for i, slot := range to.Index {
			types[i] = dictTypes[slot]
		}
		return d.call(from, types)
	case *vector.Const:
		typ := vector.TypeValueValue(to, 0)
		return d.cast(from, typ)
	case *vector.TypeValue:
		return d.call(from, to.Types())
	default:
		panic(to)
	}
}

func (d *downcast) call(from vector.Any, types []super.Type) vector.Any {
	var indexes [][]uint32
	typeToTag := make(map[super.Type]uint32)
	tags := make([]uint32, len(types))
	for i, typ := range types {
		tag, ok := typeToTag[typ]
		if !ok {
			tag = uint32(len(indexes))
			typeToTag[typ] = tag
			indexes = append(indexes, nil)
		}
		tags[i] = tag
		indexes[tag] = append(indexes[tag], uint32(i))
	}
	if len(indexes) == 1 {
		return d.cast(from, types[0])
	}
	vals := make([]vector.Any, len(indexes))
	for typ, i := range typeToTag {
		vals[i] = d.cast(vector.Pick(from, indexes[i]), typ)
	}
	return vector.NewDynamic(tags, vals)
}

func (d *downcast) cast(vec vector.Any, typ super.Type) vector.Any {
	return stripErrDowncast(d.downcast(vec, typ))
}

func (d *downcast) downcast(vec vector.Any, to super.Type) vector.Any {
	// XXX Handle vec type All.
	if _, ok := to.(*super.TypeUnion); !ok {
		if vec.Kind() == vector.KindFusion {
			return d.downcastFusion(vec, to)
		}
	}
	vec = vector.Deunion(vec)
	if dynamic, ok := vec.(*vector.Dynamic); ok {
		var vecs []vector.Any
		for _, vec := range dynamic.Values {
			vecs = append(vecs, d.downcast(vec, to))
		}
		if _, ok := to.(*super.TypeUnion); ok {
			return vector.MergeSameTypesInDynamic(d.sctx, vector.NewDynamic(dynamic.Tags, vecs))
		}
		return vector.NewDynamic(dynamic.Tags, vecs)
	}
	switch to := to.(type) {
	case *super.TypeRecord:
		return d.toRecord(vec, to)
	case *super.TypeArray:
		return d.toArray(vec, to)
	case *super.TypeSet:
		return d.toSet(vec, to)
	case *super.TypeMap:
		return d.toMap(vec, to)
	case *super.TypeUnion:
		return d.toUnion(vec, to)
	case *super.TypeError:
		return d.toError(vec, to)
	case *super.TypeNamed:
		return d.toNamed(vec, to)
	case *super.TypeFusion:
		return d.err(vector.NewWrappedError(d.sctx, "downcast: cannot downcast to a fusion type", vec))
	default:
		if vec.Type() != to {
			return d.errMismatch(vec, to)
		}
		return vec
	}
}

func (d *downcast) downcastFusion(in vector.Any, to super.Type) vector.Any {
	return vector.Apply(false, func(vecs ...vector.Any) vector.Any {
		vec := vecs[0]
		if vec.Type() != to && !isErrDowncast(vec) {
			vec = d.errSubtype(vec, to)
		}
		return vec
	}, d.derefFusion(in))
}

func (d *downcast) toRecord(vec vector.Any, to *super.TypeRecord) vector.Any {
	if vec.Kind() != vector.KindRecord {
		return d.errMismatch(vec, to)
	}
	rec := expr.PushContainerViewDown(vec).(*vector.Record)
	if len(to.Fields) == 0 {
		return vector.NewRecord(to, nil, vec.Len())
	}
	var fields []vector.Any
	for _, toField := range to.Fields {
		i, ok := rec.Typ.LUT[toField.Name]
		if !ok {
			return d.errSubtype(vec, to)
		}
		if toField.Opt && !rec.Typ.Fields[i].Opt {
			return d.errSubtype(vec, to)
		}
		fields = append(fields, d.downcast(rec.Fields[i], toField.Type))
	}
	return vector.Apply(false, func(vecs ...vector.Any) vector.Any {
		if i := slices.IndexFunc(vecs, isErrDowncast); i != -1 {
			return vecs[i]
		}
		for i, vec := range vecs {
			_, none := vec.(*vector.None)
			if none && !to.Fields[i].Opt {
				out := vector.NewRecord(rec.Typ, vecs, vecs[0].Len())
				return d.errSubtype(out, to)
			}
		}
		return vector.NewRecord(to, vecs, vecs[0].Len())
	}, fields...)
}

func (d *downcast) toArray(vec vector.Any, to *super.TypeArray) vector.Any {
	if vec.Kind() != vector.KindArray {
		return d.errMismatch(vec, to)
	}
	array := expr.PushContainerViewDown(vec).(*vector.Array)
	return d.toContainer(array.Offsets, array.Values, to, to.Type)
}

func (d *downcast) toSet(vec vector.Any, to *super.TypeSet) vector.Any {
	if vec.Kind() != vector.KindSet {
		return d.errMismatch(vec, to)
	}
	set := expr.PushContainerViewDown(vec).(*vector.Set)
	return d.toContainer(set.Offsets, set.Values, to, to.Type)
}

func (d *downcast) toContainer(offsets []uint32, inner vector.Any, to, toElem super.Type) vector.Any {
	tags, validInner, errVec := d.toList(offsets, inner, toElem)
	if errVec == nil {
		return newContainer(to, offsets, validInner)
	}
	if validInner == nil {
		return errVec
	}
	var offset uint32
	newOffsets := []uint32{offset}
	for i := range uint32(len(offsets) - 1) {
		if tags[i] == 0 {
			offset += offsets[i+1] - offsets[i]
			newOffsets = append(newOffsets, offset)
		}
	}
	vecs := []vector.Any{newContainer(to, newOffsets, validInner), errVec}
	return vector.NewDynamic(tags, vecs)
}

func (d *downcast) toMap(vec vector.Any, to *super.TypeMap) vector.Any {
	if vec.Kind() != vector.KindMap {
		return d.errMismatch(vec, to)
	}
	m := expr.PushContainerViewDown(vec).(*vector.Map)
	keyTags, keys, keyErr := d.toList(m.Offsets, m.Keys, to.KeyType)
	valTags, vals, valErr := d.toList(m.Offsets, m.Values, to.ValType)
	if keyErr == nil && valErr == nil {
		return vector.NewMap(to, m.Offsets, keys, vals)
	}
	if keyTags == nil {
		var tag uint32
		if keys == nil {
			tag = 1
		}
		keyTags = slices.Repeat([]uint32{tag}, int(m.Len()))
	}
	if valTags == nil {
		var tag uint32
		if keys == nil {
			tag = 1
		}
		valTags = slices.Repeat([]uint32{tag}, int(m.Len()))
	}
	var off uint32
	newOffsets := []uint32{off}
	var keyIndex, valIndex []uint32
	var keyCount, keyErrCount, valCount, valErrCount uint32
	var errIndexes [2][]uint32
	tags := make([]uint32, vec.Len())
	for i := range m.Len() {
		offlen := m.Offsets[i+1] - m.Offsets[i]
		keyErr, valErr := keyTags[i] == 1, valTags[i] == 1
		if keyErr || valErr {
			// If err val we need to increment (skip) the key/val counts that
			// are valid so the indexes stay correct for upcoming valid
			// keys/vals.
			var errTag uint32
			var errIndex uint32
			if !valErr {
				valCount += offlen
			} else {
				errTag = 1
				errIndex = valErrCount
				valErrCount++
			}
			if !keyErr {
				keyCount += offlen
			} else {
				errTag = 0
				errIndex = keyErrCount
				keyErrCount++
			}
			tags[i] = errTag + 1
			errIndexes[errTag] = append(errIndexes[errTag], errIndex)
			continue
		}
		for range offlen {
			keyIndex = append(keyIndex, keyCount)
			valIndex = append(valIndex, valCount)
			keyCount++
			valCount++
		}
		off += offlen
		newOffsets = append(newOffsets, off)
	}
	keyErr = d.pick(keyErr, errIndexes[0])
	valErr = d.pick(valErr, errIndexes[1])
	keys = vector.Pick(keys, keyIndex)
	vals = vector.Pick(vals, valIndex)
	nm := vector.NewMap(to, newOffsets, keys, vals)
	return vector.NewDynamic(tags, []vector.Any{nm, keyErr, valErr})
}

func (d *downcast) toList(offsets []uint32, vec vector.Any, to super.Type) ([]uint32, vector.Any, vector.Any) {
	n := uint32(len(offsets) - 1)
	vec = d.downcast(vec, to)
	dynamic, ok := vec.(*vector.Dynamic)
	if !ok {
		if isErrDowncast(vec) {
			return nil, nil, vec
		}
		return nil, vec, nil
	}
	innerTags, validVec, errVec := d.separateValidAndErrVecs(dynamic)
	if errVec == nil {
		return nil, validVec, nil
	}
	vecs := []vector.Any{validVec, errVec}
	forward := vector.NewDynamic(innerTags, vecs).ForwardTagMap()
	// If a given offset contains a single error value, then the value for that
	// slot is the first error value.
	var indexes [2][]uint32
	tags := make([]uint32, n)
	var tmpslots []uint32
	for i := range n {
		var slot, tag uint32
		tmpslots = tmpslots[:0]
		for slot = offsets[i]; slot < offsets[i+1]; slot++ {
			if innerTags[slot] == 1 {
				tag = 1
				break
			}
			tmpslots = append(tmpslots, forward[slot])
		}
		if tag == 1 {
			indexes[1] = append(indexes[1], forward[slot])
			tags[i] = 1
		} else {
			indexes[0] = append(indexes[0], tmpslots...)
		}
	}
	validVec = d.pick(validVec, indexes[0])
	errVec = d.pick(errVec, indexes[1])
	return tags, validVec, errVec
}

func (d *downcast) separateValidAndErrVecs(dynamic *vector.Dynamic) ([]uint32, vector.Any, vector.Any) {
	errTagMap := slices.Repeat([]uint32{math.MaxUint32}, len(dynamic.Values))
	validTagMap := slices.Clone(errTagMap)
	var validVecs, errVecs []vector.Any
	for i, vec := range dynamic.Values {
		if isErrDowncast(vec) {
			if vec.Len() > 0 {
				errTagMap[i] = uint32(len(errVecs))
				errVecs = append(errVecs, vec)
			}
		} else {
			validTagMap[i] = uint32(len(validVecs))
			validVecs = append(validVecs, vec)
		}
	}
	// Combine err vecs together in a single dynamic.
	var validTags, errTags []uint32
	newTags := make([]uint32, len(dynamic.Tags))
	for i, tag := range dynamic.Tags {
		if errTag := errTagMap[tag]; errTag != math.MaxUint32 {
			errTags = append(errTags, errTag)
			newTags[i] = 1
		} else {
			validTags = append(validTags, validTagMap[tag])
		}
	}
	var errs vector.Any
	switch len(errVecs) {
	case 0:
	case 1:
		errs = errVecs[0]
	default:
		errs = vector.NewDynamic(errTags, errVecs)
	}
	var valid vector.Any
	switch len(validVecs) {
	case 0:
	case 1:
		valid = validVecs[0]
	default:
		panic("system error: shouldn't have more than one valid vector in list")
	}
	return newTags, valid, errs
}

func newContainer(typ super.Type, offsets []uint32, inner vector.Any) vector.Any {
	switch typ := typ.(type) {
	case *super.TypeArray:
		return vector.NewArray(typ, offsets, inner)
	case *super.TypeSet:
		return vector.NewSet(typ, offsets, inner)
	default:
		panic(typ)
	}
}

func (d *downcast) toUnion(vec vector.Any, to *super.TypeUnion) vector.Any {
	if vec.Type() == to {
		return vec
	}
	return d.subTypeOf(vec, to.Types, func(tag int, vec vector.Any) vector.Any {
		if tag < 0 {
			if _, ok := vec.(*vector.Union); ok {
				return d.downcast(vector.Deunion(vec), to)
			}
			return d.errSubtype(vec, to)
		}
		return vector.NewUnion(to, make([]uint32, vec.Len()), []vector.Any{vec})
	})
}

func (d *downcast) toError(vec vector.Any, to *super.TypeError) vector.Any {
	if verr, ok := vec.(*vector.Error); ok {
		return d.deunion(d.downcast(verr.Vals, to.Type), func(vec vector.Any) vector.Any {
			if isErrDowncast(vec) {
				return vec
			}
			return vector.NewError(to, vec)
		})
	}
	return d.errMismatch(vec, to)
}

func (d *downcast) toNamed(vec vector.Any, to *super.TypeNamed) vector.Any {
	if fromVec, ok := vec.(*vector.Named); ok {
		if fromVec.Typ != to {
			return d.errMismatch(vec, to)
		}
		return vec
	}
	out := d.downcast(vec, to.Type)
	if isErrDowncast(out) {
		return out
	}
	return vector.NewNamed(to, out)
}

func (d *downcast) deunion(vec vector.Any, f func(vector.Any) vector.Any) vector.Any {
	return vector.Apply(true, func(vecs ...vector.Any) vector.Any {
		return f(vecs[0])
	}, vec)
}

func (d *downcast) subTypeOf(vec vector.Any, types []super.Type, f func(int, vector.Any) vector.Any) vector.Any {
	if vec.Kind() == vector.KindFusion {
		vec = d.derefFusion(vec)
		if d, ok := vec.(*vector.Dynamic); ok {
			var vals []vector.Any
			for _, val := range d.Values {
				vals = append(vals, f(slices.Index(types, val.Type()), val))
			}
			return vector.NewDynamic(d.Tags, vals)
		}
	}
	return f(slices.Index(types, vec.Type()), vec)
}

func (d *downcast) derefFusion(vec vector.Any) vector.Any {
	fusion := expr.PushContainerViewDown(vec).(*vector.Fusion)
	return d.Call(fusion.Values, fusion.Subtypes)
}

// pick is the same as vector.Pick but it strips errDowncast then reapplies it.
func (d *downcast) pick(vec vector.Any, index []uint32) vector.Any {
	if dynamic, ok := vec.(*vector.Dynamic); ok {
		return d.pickDynamic(dynamic, index)
	}
	if derr, ok := vec.(*errDowncast); ok {
		return &errDowncast{vector.Pick(derr.Any, index)}
	}
	return vector.Pick(vec, index)
}

func (d *downcast) pickDynamic(dynamic *vector.Dynamic, index []uint32) vector.Any {
	errs := make([]bool, len(dynamic.Values))
	vecs := slices.Clone(dynamic.Values)
	for i, vec := range vecs {
		if derr, ok := vec.(*errDowncast); ok {
			vecs[i] = derr.Any
			errs[i] = true
		}
	}
	dynamic = vector.Pick(vector.NewDynamic(dynamic.Tags, vecs), index).(*vector.Dynamic)
	for i, vec := range dynamic.Values {
		if errs[i] {
			dynamic.Values[i] = &errDowncast{vec}
		}
	}
	return dynamic
}

type errDowncast struct {
	vector.Any
}

func stripErrDowncast(vec vector.Any) vector.Any {
	if dynamic, ok := vec.(*vector.Dynamic); ok {
		vecs := slices.Clone(dynamic.Values)
		for i, vec := range vecs {
			vecs[i] = stripErrDowncast(vec)
		}
		return vector.NewDynamic(dynamic.Tags, vecs)
	}
	if derr, ok := vec.(*errDowncast); ok {
		return derr.Any
	}
	return vec
}

func isErrDowncast(vec vector.Any) bool {
	_, ok := vec.(*errDowncast)
	return ok
}

func (d *downcast) errMismatch(vec vector.Any, to super.Type) vector.Any {
	err := vector.NewWrappedError(d.sctx, "downcast: type mismatch to "+sup.FormatType(to), vec)
	return d.err(err)
}

func (d *downcast) errSubtype(vec vector.Any, to super.Type) vector.Any {
	err := vector.NewWrappedError(d.sctx, "downcast: invalid subtype "+sup.FormatType(to), vec)
	return d.err(err)
}

func (d *downcast) err(vec vector.Any) vector.Any {
	return &errDowncast{vec}
}
