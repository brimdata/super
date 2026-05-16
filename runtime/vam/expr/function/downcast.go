package function

import (
	"fmt"

	"github.com/brimdata/super"
	samfunc "github.com/brimdata/super/runtime/sam/expr/function"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/vbuild"
)

type downcast struct {
	sctx    *super.Context
	defuser *defuse
}

func newDowncast(sctx *super.Context) *downcast {
	return newDefuse(sctx).downcast
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
		vec, _ := d.defuseWithErrors(from, types)
		return vec
	case *vector.Dict:
		dictTypes := to.Any.(*vector.TypeValue).Types()
		types := make([]super.Type, len(to.Index))
		for i, slot := range to.Index {
			types[i] = dictTypes[slot]
		}
		vec, _ := d.defuseWithErrors(from, types)
		return vec
	case *vector.Const:
		typ := vector.TypeValueValue(to, 0)
		return d.cast(from, typ)
	case *vector.TypeValue:
		vec, _ := d.defuseWithErrors(from, to.Types())
		return vec
	default:
		panic(to)
	}
}

func (d *downcast) defuseWithErrors(from vector.Any, types []super.Type) (vector.Any, bool) {
	vec, err := d.defuse(from, types)
	if err != nil {
		return err, false
	}
	return vec, true
}

func (d *downcast) defuse(from vector.Any, types []super.Type) (vector.Any, vector.Any) {
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
		return d.downcast(from, types[0])
	}
	vals := make([]vector.Any, len(indexes))
	var errs int
	for typ, i := range typeToTag {
		vec, err := d.downcast(vector.Pick(from, indexes[i]), typ)
		if err != nil {
			errs++
			vals[i] = err
		} else {
			vals[i] = vec
		}
	}
	out := vector.NewDynamic(tags, vals)
	if errs != 0 {
		return nil, out
	}
	return out, nil
}

func (d *downcast) cast(vec vector.Any, typ super.Type) vector.Any {
	vec, err := d.downcast(vec, typ)
	if err != nil {
		return err
	}
	return vec
}

func (d *downcast) downcast(vec vector.Any, to super.Type) (vector.Any, vector.Any) {
	fmt.Println("=======")
	fmt.Println("TO", sup.String(to))
	vector.Println(vec)
	val, err := d.downcast0(vec, to)
	if err != nil {
		fmt.Println("GOT ERR")
		vector.Println(err)
	}
	if val != nil {
		fmt.Println("GOT VAL")
		vector.Println(val)
	}
	return val, err
}

func (d *downcast) downcast0(vec vector.Any, to super.Type) (vector.Any, vector.Any) {
	// XXX Handle vec type All.
	if _, ok := to.(*super.TypeUnion); !ok {
		if _, ok := vec.(*vector.Fusion); ok {
			fusion := expr.PushContainerViewDown(vec).(*vector.Fusion)
			return d.defuse(fusion.Values, fusion.Subtypes.Types())
		}
	}
	vec = vector.Deunion(vec)
	if dynamic, ok := vec.(*vector.Dynamic); ok {
		var vecs []vector.Any
		for _, vec := range dynamic.Values {
			vecs = append(vecs, d.cast(vec, to))
		}
		if _, ok := to.(*super.TypeUnion); ok {
			return vbuild.MergeSameTypesInDynamic(d.sctx, vector.NewDynamic(dynamic.Tags, vecs)), nil
		}
		if len(vecs) == 1 {
			return vecs[0], nil
		}
		return vector.NewDynamic(dynamic.Tags, vecs), nil
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
	case *super.TypeEnum:
		return d.toEnum(vec, to)
	case *super.TypeError:
		return d.toError(vec, to)
	case *super.TypeNamed:
		return d.toNamed(vec, to)
	case *super.TypeFusion:
		return nil, vector.NewWrappedError(d.sctx, "downcast: cannot downcast to a fusion type", vec)
	default:
		if vec.Type() != to {
			if vec.Type() == super.TypeNone {
				return nil, d.errNonOptionNone(vec, to)
			}
			return nil, d.errMismatch(vec, to)
		}
		return vec, nil
	}
}

func (d *downcast) toRecord(vec vector.Any, to *super.TypeRecord) (vector.Any, vector.Any) {
	if vec.Kind() != vector.KindRecord {
		return nil, d.errMismatch(vec, to)
	}
	rec := expr.PushContainerViewDown(vec).(*vector.Record)
	if len(to.Fields) == 0 {
		return vector.NewRecord(to, nil, vec.Len()), nil
	}
	var fields []vector.Any
	for _, toField := range to.Fields {
		i, ok := rec.Typ.LUT[toField.Name]
		if !ok {
			return nil, d.errSubtype(vec, to)
		}
		if super.IsOptionType(toField.Type) {
			fromFieldType := rec.Typ.Fields[i].Type
			if f, ok := fromFieldType.(*super.TypeFusion); ok {
				fromFieldType = f.Type
			}
			if !super.IsOptionType(fromFieldType) {
				fmt.Println("REC OPT ERR")
				return nil, d.errSubtype(vec, to)
			}
		}
		vec, err := d.downcast(rec.Fields[i], toField.Type)
		fmt.Println("REC FIELD", vec != nil, err != nil)
		if err != nil {
			return nil, err
		}
		fields = append(fields, vec)
	}
	return vector.NewRecord(to, fields, fields[0].Len()), nil
}

func (d *downcast) toArray(vec vector.Any, to *super.TypeArray) (vector.Any, vector.Any) {
	if vec.Kind() != vector.KindArray {
		return nil, d.errMismatch(vec, to)
	}
	array := expr.PushContainerViewDown(vec).(*vector.Array)
	return d.toContainer(array.Offsets, array.Values, to, to.Type)
}

func (d *downcast) toSet(vec vector.Any, to *super.TypeSet) (vector.Any, vector.Any) {
	if vec.Kind() != vector.KindSet {
		return nil, d.errMismatch(vec, to)
	}
	set := expr.PushContainerViewDown(vec).(*vector.Set)
	return d.toContainer(set.Offsets, set.Values, to, to.Type)
}

func (d *downcast) toContainer(offsets []uint32, inner vector.Any, to, toElem super.Type) (vector.Any, vector.Any) {
	inner, err := d.downcast(inner, toElem)
	if err != nil {
		return nil, err
	}
	fmt.Println("=== [TO CONTAINER] === ")
	vector.Println(inner)
	fmt.Println("=== [OUT CONTAINER] === !!! ")
	switch to := to.(type) {
	case *super.TypeArray:
		return vector.NewArray(to, offsets, inner), nil
	case *super.TypeSet:
		return vector.NewSet(to, offsets, inner), nil
	default:
		panic(to)
	}
}

func (d *downcast) toMap(vec vector.Any, to *super.TypeMap) (vector.Any, vector.Any) {
	if vec.Kind() != vector.KindMap {
		return nil, d.errMismatch(vec, to)
	}
	m := expr.PushContainerViewDown(vec).(*vector.Map)
	keys, err := d.downcast(m.Keys, to.KeyType)
	if err != nil {
		return nil, err
	}
	vals, err := d.downcast(m.Values, to.ValType)
	if err != nil {
		return nil, err
	}
	return vector.NewMap(to, m.Offsets, keys, vals), nil
}

func (d *downcast) toUnion(vec vector.Any, to *super.TypeUnion) (vector.Any, vector.Any) {
	if vec.Type() == to {
		return vec, nil
	}
	vec, ok := d.defuser.eval(vec)
	if !ok {
		fmt.Println(" !!!!!!!!!!!!!!! NOT OK!!!!")
		vector.Println(vec)
		return nil, vec
	}
	dyn, ok := vec.(*vector.Dynamic)
	if !ok {
		tag := samfunc.DowncastSubtypeIndex(to.Types, vec.Type())
		if tag < 0 {
			return nil, d.errSubtype(vec, to)
		}
		tags := make([]uint32, vec.Len())
		fmt.Println("UNION OK!!!")
		return vector.NewUnion(to, tags, []vector.Any{vec}), nil
	}
	var vals []vector.Any
	tagmap := make([]uint32, len(dyn.Values))
	var errs int
	for i, val := range dyn.Values {
		if val != nil {
			tag := samfunc.DowncastSubtypeIndex(to.Types, val.Type())
			if tag < 0 {
				//XXX this error isn't right but it will due until
				// we get this working.  We`` can fix by changing the
				// vector.Error return value to Any and mixing valid
				// values in the error position with the errors
				val = d.errSubtype(val, to)
				errs++
			}
			tagmap[i] = uint32(len(vals))
			vals = append(vals, val)
		}
	}
	tags := make([]uint32, len(dyn.Tags))
	for k := range tags {
		tags[k] = tagmap[dyn.Tags[k]]
	}
	if errs != 0 {
		var types []super.Type
		for _, vec := range vals {
			types = append(types, vec.Type())
		}
		//XXX merge same types?
		if len(types) != len(super.UniqueTypes(types)) {
			panic(".")
		}
		errType, ok := d.sctx.LookupTypeUnion(types)
		if !ok {
			panic(types)
		}
		fmt.Println("UNION ERR!!!")
		return nil, vector.NewUnion(errType, tags, vals)
	}
	fmt.Println("UNION NO ERR!!!")
	return vector.NewUnion(to, tags, vals), nil
}

func (d *downcast) toEnum(vec vector.Any, to *super.TypeEnum) (vector.Any, vector.Any) {
	origVec := vec
	var index []uint32
	if view, ok := vec.(*vector.View); ok {
		vec = view.Any
		index = view.Index
	}
	enumVec, ok := vec.(*vector.Enum)
	if !ok {
		return nil, d.errMismatch(origVec, to)
	}
	indexes := make([]uint64, origVec.Len())
	for i := range indexes {
		j := uint32(i)
		if index != nil {
			j = index[j]
		}
		fromIndex := enumVec.Uint.Values[j]
		symbol, err := enumVec.Typ.Symbol(int(fromIndex))
		if err != nil {
			return nil, d.errMismatch(origVec, to)
		}
		toIndex := to.Lookup(symbol)
		if toIndex < 0 {
			return nil, d.errMismatch(origVec, to)
		}
		indexes[i] = uint64(toIndex)
	}
	return vector.NewEnum(to, indexes), nil
}

func (d *downcast) toError(vec vector.Any, to *super.TypeError) (vector.Any, vector.Any) {
	if vec.Kind() != vector.KindMap {
		return nil, d.errMismatch(vec, to)
	}
	return vector.NewError(to, vec), nil
}

func (d *downcast) toNamed(vec vector.Any, to *super.TypeNamed) (vector.Any, vector.Any) {
	if fromVec, ok := vec.(*vector.Named); ok {
		if fromVec.Typ != to {
			return nil, d.errMismatch(vec, to)
		}
		return vec, nil
	}
	//XXX don't think we need this recursion because named are now a barrier to fusion
	out, err := d.downcast(vec, to.Type)
	if err != nil {
		return nil, err
	}
	return vector.NewNamed(to, out), nil
}

func (d *downcast) errNonOptionNone(vec vector.Any, to super.Type) *vector.Error {
	return vector.NewStringError(d.sctx, "downcast: none value in non-option type: "+sup.FormatType(to), vec.Len())
}

func (d *downcast) errMismatch(vec vector.Any, to super.Type) *vector.Error {
	return vector.NewWrappedError(d.sctx, "downcast: type mismatch to "+sup.FormatType(to), vec)
}

func (d *downcast) errSubtype(vec vector.Any, to super.Type) *vector.Error {
	return vector.NewWrappedError(d.sctx, "downcast: invalid subtype "+sup.FormatType(to), vec)
}
