package jsonvec

import (
	"encoding/binary"
	"strings"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
)

func Materialize(sctx *super.Context, b *Builder) vector.Any {
	return vector.Apply(true, func(vecs ...vector.Any) vector.Any {
		return expr.Unblend(sctx, vecs[0])
	}, materialize(sctx, b.stack[0]))
}

func materialize(sctx *super.Context, v Value) vector.Any {
	switch v := v.(type) {
	case *Null:
		return vector.NewNull(v.len)
	case *Bool:
		return vector.NewBool(v.Value)
	case *Int:
		return v.Value
	case *Float:
		return v.Value
	case *String:
		return v.Value
	case *Union:
		return materializeUnion(sctx, v)
	case *Array:
		inner := materialize(sctx, v.Inner)
		typ := sctx.LookupTypeArray(inner.Type())
		return vector.NewArray(typ, v.Offsets, inner)
	case *Record:
		return materializeRecord(sctx, v)
	default:
		panic(v)
	}
}

func materializeUnion(sctx *super.Context, u *Union) vector.Any {
	var types []super.Type
	var vecs []vector.Any
	for _, v := range u.Values() {
		vec := materialize(sctx, v)
		types = append(types, vec.Type())
		vecs = append(vecs, vec)
	}
	subTypes := make([]super.Type, len(u.Tags))
	for i, tag := range u.Tags {
		subTypes[i] = types[tag]
	}
	utyp := sctx.LookupTypeUnion(types)
	return vector.NewUnion(utyp, u.Tags, vecs)
}

func materializeRecord(sctx *super.Context, r *Record) vector.Any {
	fieldNames := make([]string, len(r.LUT))
	for name, id := range r.LUT {
		fieldNames[id] = name
	}
	n := r.Len()
	var vecs []vector.Any
	var allFields []super.Field
	for i, field := range r.Fields {
		rle := r.RLEs[i].End(n)
		vec := materialize(sctx, field.Value)
		field := super.NewFieldWithOpt(fieldNames[i], vec.Type(), len(rle) > 0)
		vecs = append(vecs, vector.NewFieldFromRLE(sctx, vec, n, rle))
		allFields = append(allFields, field)
	}
	rtyp := sctx.MustLookupTypeRecord(allFields)
	record := vector.NewRecord(rtyp, vecs, n)
	if len(r.typeToTag) == 1 {
		return record
	}
	subTypeFieldIds := make([][]int, len(r.typeToTag))
	subTypeMap := make([]*super.TypeRecord, len(r.typeToTag))
	for desc, tag := range r.typeToTag {
		r := strings.NewReader(desc)
		var subFields []super.Field
		var fieldIds []int
		for {
			fieldId, err := binary.ReadUvarint(r)
			if err != nil {
				break
			}
			fieldIds = append(fieldIds, int(fieldId))
			f := allFields[fieldId]
			subFields = append(subFields, super.NewField(f.Name, f.Type))
		}
		subTypeFieldIds[tag] = fieldIds
		subTypeMap[tag] = sctx.MustLookupTypeRecord(subFields)
	}
	indexes := make([][]uint32, len(subTypeMap))
	dtags := make([]uint32, n)
	for i, tag := range r.tags {
		indexes[tag] = append(indexes[tag], uint32(i))
		dtags[i] = tag
	}
	var recs []vector.Any
	for i, typ := range subTypeMap {
		var subvecs []vector.Any
		for _, fieldId := range subTypeFieldIds[i] {
			subvecs = append(subvecs, vector.Pick(vecs[fieldId], indexes[i]))
		}
		recs = append(recs, vector.NewRecord(typ, subvecs, uint32(len(indexes[i]))))
	}
	return vector.NewUnionFromDynamic(sctx, vector.NewDynamic(dtags, recs))
}
