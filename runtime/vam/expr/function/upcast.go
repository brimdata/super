package function

import (
	"slices"

	"github.com/brimdata/super"
	samfunc "github.com/brimdata/super/runtime/sam/expr/function"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
)

type Upcast struct {
	sctx      *super.Context
	typeToTag map[super.Type]uint32
}

func NewUpcast(sctx *super.Context) *Upcast {
	return &Upcast{sctx, map[super.Type]uint32{}}
}

func (u *Upcast) Call(args ...vector.Any) vector.Any {
	from, to := args[0], args[1]
	if to.Kind() != vector.KindType {
		return vector.NewWrappedError(u.sctx, "upcast: type argument not a type", to)
	}
	if to, ok := to.(*vector.Const); ok {
		return u.upcastOrError(from, vector.TypeValueValue(to, 0))
	}
	var indexes [][]uint32
	var tags []uint32
	for i := range to.Len() {
		typ := vector.TypeValueValue(to, i)
		tag, ok := u.typeToTag[typ]
		if !ok {
			tag = uint32(len(u.typeToTag))
			u.typeToTag[typ] = tag
		}
		if len(u.typeToTag) == 1 {
			// There's only one type so we don't need indexes or tags.
			continue
		}
		if len(indexes) == 0 {
			index := make([]uint32, i)
			for i := range i {
				index[i] = i
			}
			indexes = [][]uint32{index}
			tags = make([]uint32, i)
		}
		if !ok {
			indexes = append(indexes, nil)
		}
		indexes[tag] = append(indexes[tag], i)
		tags = append(tags, tag)
	}
	defer clear(u.typeToTag)
	if len(u.typeToTag) == 1 {
		return u.upcastOrError(from, vector.TypeValueValue(to, 0))
	}
	vecs := make([]vector.Any, len(u.typeToTag))
	for typ, tag := range u.typeToTag {
		vecs[tag] = u.upcastOrError(vector.Pick(from, indexes[tag]), typ)
	}
	return vector.NewDynamic(tags, vecs)
}

func (u *Upcast) upcastOrError(vec vector.Any, typ super.Type) vector.Any {
	out := u.upcast(vec, typ)
	if out == nil {
		out = vector.NewWrappedError(u.sctx, "upcast: value not a subtype of "+sup.FormatType(typ), vec)
	}
	return out
}

func (u *Upcast) Cast(vec vector.Any, to super.Type) (vector.Any, bool) {
	out := u.upcast(vec, to)
	return out, out != nil
}

func (u *Upcast) upcast(vec vector.Any, to super.Type) vector.Any {
	if vec.Type() == to {
		return vec
	}
	switch vec := vec.(type) {
	case *vector.Const:
		vec2 := u.upcast(vec.Any, to)
		if vec2 == nil {
			return nil
		}
		return vector.NewConst(vec2, vec.Len())
	case *vector.Dict:
		vec2 := u.upcast(vec.Any, to)
		if vec2 == nil {
			return nil
		}
		return vector.NewDict(vec2, vec.Index, vec.Counts)
	case *vector.View:
		vec2 := u.upcast(vec.Any, to)
		if vec2 == nil {
			return nil
		}
		return vector.Pick(vec2, vec.Index)
	}
	switch to := to.(type) {
	case *super.TypeRecord:
		return u.toRecord(vec, to)
	case *super.TypeArray:
		return u.toArray(vec, to)
	case *super.TypeSet:
		return u.toSet(vec, to)
	case *super.TypeMap:
		return u.toMap(vec, to)
	case *super.TypeUnion:
		return u.toUnion(vec, to)
	case *super.TypeError:
		return u.toError(vec, to)
	case *super.TypeNamed:
		return u.toNamed(vec, to)
	case *super.TypeFusion:
		return u.toFusion(vec, to)
	default:
		return nil
	}
}

func (u *Upcast) toRecord(vec vector.Any, to *super.TypeRecord) vector.Any {
	recVec, ok := vec.(*vector.Record)
	if !ok {
		return nil
	}
	fieldVecs := make([]vector.Any, len(to.Fields))
	for i, f := range to.Fields {
		n, ok := recVec.Typ.IndexOfField(f.Name)
		if !ok {
			if !f.Opt {
				return nil
			}
			fieldVecs[i] = vector.NewNone(u.sctx, f.Type, vec.Len())
			continue
		}
		fieldVecs[i] = u.upcast(recVec.Fields[n], f.Type)
		if fieldVecs[i] == nil {
			return nil
		}
	}
	return vector.NewRecord(to, fieldVecs, vec.Len())
}

func (u *Upcast) toArray(vec vector.Any, to *super.TypeArray) vector.Any {
	arrVec, ok := vec.(*vector.Array)
	if !ok {
		return nil
	}
	values := u.deunionAndUpcast(arrVec.Values, to.Type)
	if values == nil {
		return nil
	}
	return vector.NewArray(to, arrVec.Offsets, values)
}

func (u *Upcast) toSet(vec vector.Any, to *super.TypeSet) vector.Any {
	setVec, ok := vec.(*vector.Set)
	if !ok {
		return nil
	}
	values := u.deunionAndUpcast(setVec.Values, to.Type)
	if values == nil {
		return nil
	}
	return vector.NewSet(to, setVec.Offsets, values)
}

func (u *Upcast) deunionAndUpcast(vec vector.Any, to super.Type) vector.Any {
	d, ok := vector.Deunion(vec).(*vector.Dynamic)
	if !ok {
		return u.upcast(vec, to)
	}
	vecs := slices.Clone(d.Values)
	for i, vec := range vecs {
		if vec == nil || vec.Len() == 0 {
			continue
		}
		vecs[i] = u.upcast(vecs[i], to)
		if vecs[i] == nil {
			return nil
		}
	}
	return vector.MergeSameTypesInDynamic(u.sctx, vector.NewDynamic(d.Tags, vecs))
}

func (u *Upcast) toMap(vec vector.Any, to *super.TypeMap) vector.Any {
	mapVec, ok := vec.(*vector.Map)
	if !ok {
		return nil
	}
	keys := u.upcast(mapVec.Keys, to.KeyType)
	if keys == nil {
		return nil
	}
	values := u.upcast(mapVec.Values, to.ValType)
	if values == nil {
		return nil
	}
	return vector.NewMap(to, mapVec.Offsets, keys, values)
}

func (u *Upcast) toUnion(vec vector.Any, to *super.TypeUnion) vector.Any {
	if unionVec, ok := vec.(*vector.Union); ok {
		values := make([]vector.Any, len(unionVec.Values))
		for i, vec := range unionVec.Values {
			values[i] = u.toUnionValue(vec, to)
			if values[i] == nil {
				return nil
			}
		}
		return vector.NewUnion(to, unionVec.Tags, values)
	}
	values := u.toUnionValue(vec, to)
	if values == nil {
		return nil
	}
	tags := make([]uint32, vec.Len())
	return vector.NewUnion(to, tags, []vector.Any{values})
}

func (u *Upcast) toUnionValue(vec vector.Any, to *super.TypeUnion) vector.Any {
	tag := samfunc.UpcastUnionTag(to.Types, vec.Type())
	if tag < 0 {
		return nil
	}
	return u.upcast(vec, to.Types[tag])
}

func (u *Upcast) toError(vec vector.Any, to *super.TypeError) vector.Any {
	errVec, ok := vec.(*vector.Error)
	if !ok {
		return nil
	}
	values := u.upcast(errVec.Vals, to.Type)
	if values == nil {
		return nil
	}
	return vector.NewError(to, values)
}

func (u *Upcast) toNamed(vec vector.Any, to *super.TypeNamed) vector.Any {
	namedVec, ok := vec.(*vector.Named)
	if !ok {
		return nil
	}
	vec = u.upcast(namedVec.Any, to.Type)
	if vec == nil {
		return nil
	}
	return vector.NewNamed(to, vec)
}

func (u *Upcast) toFusion(vec vector.Any, to *super.TypeFusion) vector.Any {
	values := u.upcast(vec, to.Type)
	if values == nil {
		return nil
	}
	typ := vec.Type()
	subtypes := make([]super.Type, vec.Len())
	for i := range subtypes {
		subtypes[i] = typ
	}
	return vector.NewFusion(u.sctx, to, values, subtypes)
}
