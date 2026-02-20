package function

import (
	"maps"
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/sup"
)

type Caster interface {
	Cast(from super.Value, to super.Type) super.Value
}

type cast struct {
	sctx *super.Context
}

func NewCaster(sctx *super.Context) Caster {
	return &cast{sctx: sctx}
}

func (c *cast) Call(args []super.Value) super.Value {
	from, to := args[0], args[1]
	if from.IsError() {
		return from
	}
	switch toUnder := to.Under(); toUnder.Type().ID() {
	case super.IDString:
		typ, err := c.sctx.LookupTypeNamed(toUnder.AsString(), super.TypeUnder(from.Type()))
		if err != nil {
			return c.sctx.WrapError("cannot cast to named type: "+err.Error(), from)
		}
		return super.NewValue(typ, from.Bytes())
	case super.IDType:
		typ, err := c.sctx.LookupByValue(toUnder.Bytes())
		if err != nil {
			panic(err)
		}
		return c.Cast(from, typ)
	}
	return c.sctx.WrapError("cast target must be a type or type name", to)
}

func (c *cast) Cast(from super.Value, to super.Type) super.Value {
	switch fromType := from.Type(); {
	case fromType == to:
		return from
	case fromType.ID() == to.ID():
		return super.NewValue(to, from.Bytes())
	}
	switch to := to.(type) {
	case *super.TypeRecord:
		return c.toRecord(from, to)
	case *super.TypeArray, *super.TypeSet:
		return c.toArrayOrSet(from, to)
	case *super.TypeMap:
		return c.toMap(from, to)
	case *super.TypeUnion:
		return c.toUnion(from, to)
	case *super.TypeError:
		return c.toError(from, to)
	case *super.TypeNamed:
		return c.toNamed(from, to)
	default:
		from = from.Under()
		caster := expr.LookupPrimitiveCaster(c.sctx, to)
		if caster == nil {
			return c.error(from, to)
		}
		return caster.Eval(from)
	}
}

func (c *cast) error(from super.Value, to super.Type) super.Value {
	return c.sctx.WrapError("cannot cast to "+sup.FormatType(to), from)
}

func (c *cast) toRecord(from super.Value, to *super.TypeRecord) super.Value {
	from = from.Under()
	if !super.IsRecordType(from.Type()) {
		return c.error(from, to)
	}
	var b scode.Builder
	var fields []super.Field
	for i, f := range to.Fields {
		var val2 super.Value
		if fieldVal := from.Deref(f.Name); fieldVal != nil {
			val2 = c.Cast(*fieldVal, f.Type)
		} else {
			val2 = c.sctx.Missing()
		}
		if t := val2.Type(); t != f.Type {
			if fields == nil {
				fields = slices.Clone(to.Fields)
			}
			fields[i].Type = t
		}
		b.Append(val2.Bytes())
	}
	if fields != nil {
		to = c.sctx.MustLookupTypeRecord(fields)
	}
	return super.NewValue(to, b.Bytes())
}

func (c *cast) toArrayOrSet(from super.Value, to super.Type) super.Value {
	from = from.Under()
	fromInner := super.InnerType(from.Type())
	toInner := super.InnerType(to)
	if fromInner == nil {
		// XXX Should also return an error if casting from fromInner to
		// toInner will always fail.
		return c.error(from, to)
	}
	types := map[super.Type]struct{}{}
	var vals []super.Value
	for it := from.Iter(); !it.Done(); {
		val := c.castNext(&it, fromInner, toInner)
		types[val.Type()] = struct{}{}
		vals = append(vals, val)
	}
	if len(vals) == 0 {
		return super.NewValue(to, from.Bytes())
	}
	inner := c.maybeConvertToUnion(vals, types)
	if inner != toInner {
		if to.Kind() == super.ArrayKind {
			to = c.sctx.LookupTypeArray(inner)
		} else {
			to = c.sctx.LookupTypeSet(inner)
		}
	}
	var bytes scode.Bytes
	for _, val := range vals {
		bytes = scode.Append(bytes, val.Bytes())
	}
	if to.Kind() == super.SetKind {
		bytes = super.NormalizeSet(bytes)
	}
	return super.NewValue(to, bytes)
}

func (c *cast) castNext(it *scode.Iter, from, to super.Type) super.Value {
	val := super.NewValue(from, it.Next())
	return c.Cast(val, to)
}

func (c *cast) maybeConvertToUnion(vals []super.Value, types map[super.Type]struct{}) super.Type {
	typesSlice := slices.Collect(maps.Keys(types))
	if len(typesSlice) == 1 {
		return typesSlice[0]
	}
	union := c.sctx.LookupTypeUnion(typesSlice)
	for i, val := range vals {
		vals[i] = c.toUnion(val, union)
	}
	return union
}

func (c *cast) toMap(from super.Value, to *super.TypeMap) super.Value {
	from = from.Under()
	fromType, ok := from.Type().(*super.TypeMap)
	if !ok {
		return c.error(from, to)
	}
	keyTypes := map[super.Type]struct{}{}
	valTypes := map[super.Type]struct{}{}
	var keyVals, valVals []super.Value
	for it := from.Iter(); !it.Done(); {
		keyVal := c.castNext(&it, fromType.KeyType, to.KeyType)
		keyVals = append(keyVals, keyVal)
		keyTypes[keyVal.Type()] = struct{}{}
		valVal := c.castNext(&it, fromType.ValType, to.ValType)
		valTypes[valVal.Type()] = struct{}{}
		valVals = append(valVals, valVal)
	}
	if len(keyVals) == 0 {
		return super.NewValue(to, from.Bytes())
	}
	keyType := c.maybeConvertToUnion(keyVals, keyTypes)
	valType := c.maybeConvertToUnion(valVals, valTypes)
	if keyType != to.KeyType || valType != to.ValType {
		to = c.sctx.LookupTypeMap(keyType, valType)
	}
	var bytes scode.Bytes
	for i := range keyVals {
		bytes = scode.Append(bytes, keyVals[i].Bytes())
		bytes = scode.Append(bytes, valVals[i].Bytes())
	}
	return super.NewValue(to, super.NormalizeMap(bytes))
}

func (c *cast) toUnion(from super.Value, to *super.TypeUnion) super.Value {
	tag := bestUnionTag(from.Type(), to)
	if tag < 0 {
		from2 := from.Deunion()
		tag = bestUnionTag(from2.Type(), to)
		if tag < 0 {
			return c.error(from, to)
		}
		from = from2
	}
	var b scode.Builder
	super.BuildUnion(&b, tag, from.Bytes())
	return super.NewValue(to, b.Bytes().Body())
}

func (c *cast) toError(from super.Value, to *super.TypeError) super.Value {
	from = c.Cast(from, to.Type)
	if from.Type() != to.Type {
		return from
	}
	return super.NewValue(to, from.Bytes())
}

func (c *cast) toNamed(from super.Value, to *super.TypeNamed) super.Value {
	from = c.Cast(from, to.Type)
	if from.Type() != to.Type {
		return from
	}
	return super.NewValue(to, from.Bytes())
}

type upcast struct {
	sctx *super.Context
}

func NewUpCaster(sctx *super.Context) Caster {
	return &upcast{sctx: sctx}
}

func (u *upcast) Call(args []super.Value) super.Value {
	from, to := args[0], args[1]
	if from.IsError() {
		//XXX wrap?
		return from
	}
	if _, ok := super.TypeUnder(to.Type()).(*super.TypeOfType); !ok {
		return u.sctx.WrapError("upcast target must be a type", to)
	}
	typ, err := u.sctx.LookupByValue(to.Bytes())
	if err != nil {
		panic(err)
	}
	return u.Cast(from, typ)
}

func (u *upcast) Cast(from super.Value, to super.Type) super.Value {
	switch fromType := from.Type(); {
	case fromType == to:
		return from
	case fromType.ID() == to.ID():
		return super.NewValue(to, from.Bytes())
	}
	switch to := to.(type) {
	case *super.TypeRecord:
		val, _ := u.toRecord(from, to)
		return val
	case *super.TypeArray, *super.TypeSet:
		return u.toArrayOrSet(from, to)
	case *super.TypeMap:
		return u.toMap(from, to)
	case *super.TypeUnion:
		return u.toUnion(from, to)
	case *super.TypeError:
		return u.toError(from, to)
	case *super.TypeNamed:
		return u.toNamed(from, to)
	default:
		return u.error(from, to)
	}
}

func (u *upcast) error(from super.Value, to super.Type) super.Value {
	return u.sctx.WrapError("cannot upcast to "+sup.FormatType(to), from)
}

func (u *upcast) toRecord(from super.Value, to *super.TypeRecord) (super.Value, bool) {
	from = from.Under()
	if !super.IsRecordType(from.Type()) {
		return u.error(from, to), false
	}
	var b scode.Builder
	var fields []super.Field
	for i, f := range to.Fields {
		var val2 super.Value
		if fieldVal := from.Deref(f.Name); fieldVal != nil {
			val2 = u.Cast(*fieldVal, f.Type)
		} else {
			// The field is present in the top but not the value.
			// If the type is nullable, encode this as null (XXX this will
			// change to None in an optional field) in the optional-fields PR.
			if union, tag := super.NullableUnion(f.Type); union != nil {
				super.BuildUnion(&b, tag, nil)
				if fields == nil {
					fields = slices.Clone(to.Fields)
				}
				fields[i].Type = union
				continue
			} else {
				val2 = u.sctx.Missing()
			}
		}
		if t := val2.Type(); t != f.Type {
			if fields == nil {
				fields = slices.Clone(to.Fields)
			}
			fields[i].Type = t
		}
		b.Append(val2.Bytes())
	}
	if fields != nil {
		to = u.sctx.MustLookupTypeRecord(fields)
	}
	return super.NewValue(to, b.Bytes()), true
}

func (u *upcast) toArrayOrSet(from super.Value, to super.Type) super.Value {
	from = from.Under()
	fromInner := super.InnerType(from.Type())
	toInner := super.InnerType(to)
	if fromInner == nil {
		// XXX Should also return an error if casting from fromInner to
		// toInner will always fail.
		return u.error(from, to)
	}
	types := map[super.Type]struct{}{}
	var vals []super.Value
	for it := from.Iter(); !it.Done(); {
		val := u.castNext(&it, fromInner, toInner)
		types[val.Type()] = struct{}{}
		vals = append(vals, val)
	}
	if len(vals) == 0 {
		return super.NewValue(to, from.Bytes())
	}
	inner := u.maybeConvertToUnion(vals, types)
	if inner != toInner {
		if to.Kind() == super.ArrayKind {
			to = u.sctx.LookupTypeArray(inner)
		} else {
			to = u.sctx.LookupTypeSet(inner)
		}
	}
	var bytes scode.Bytes
	for _, val := range vals {
		bytes = scode.Append(bytes, val.Bytes())
	}
	if to.Kind() == super.SetKind {
		bytes = super.NormalizeSet(bytes)
	}
	return super.NewValue(to, bytes)
}

func (u *upcast) castNext(it *scode.Iter, from, to super.Type) super.Value {
	val := super.NewValue(from, it.Next())
	return u.Cast(val, to)
}

func (u *upcast) maybeConvertToUnion(vals []super.Value, types map[super.Type]struct{}) super.Type {
	typesSlice := slices.Collect(maps.Keys(types))
	if len(typesSlice) == 1 {
		return typesSlice[0]
	}
	union := u.sctx.LookupTypeUnion(typesSlice)
	for i, val := range vals {
		vals[i] = u.toUnion(val, union)
	}
	return union
}

func (u *upcast) toMap(from super.Value, to *super.TypeMap) super.Value {
	from = from.Under()
	fromType, ok := from.Type().(*super.TypeMap)
	if !ok {
		return u.error(from, to)
	}
	keyTypes := map[super.Type]struct{}{}
	valTypes := map[super.Type]struct{}{}
	var keyVals, valVals []super.Value
	for it := from.Iter(); !it.Done(); {
		keyVal := u.castNext(&it, fromType.KeyType, to.KeyType)
		keyVals = append(keyVals, keyVal)
		keyTypes[keyVal.Type()] = struct{}{}
		valVal := u.castNext(&it, fromType.ValType, to.ValType)
		valTypes[valVal.Type()] = struct{}{}
		valVals = append(valVals, valVal)
	}
	if len(keyVals) == 0 {
		return super.NewValue(to, from.Bytes())
	}
	keyType := u.maybeConvertToUnion(keyVals, keyTypes)
	valType := u.maybeConvertToUnion(valVals, valTypes)
	if keyType != to.KeyType || valType != to.ValType {
		to = u.sctx.LookupTypeMap(keyType, valType)
	}
	var bytes scode.Bytes
	for i := range keyVals {
		bytes = scode.Append(bytes, keyVals[i].Bytes())
		bytes = scode.Append(bytes, valVals[i].Bytes())
	}
	return super.NewValue(to, super.NormalizeMap(bytes))
}

func (u *upcast) toUnion(from super.Value, to *super.TypeUnion) super.Value {
	from = from.Deunion()
	tag := upcastUnionTag(to.Types, from.Type())
	if tag < 0 {
		return u.error(from, to)
	}
	tagType := to.Types[tag]
	from = u.Cast(from, tagType)
	if from.Type() != tagType {
		return from
	}
	var b scode.Builder
	super.BuildUnion(&b, tag, from.Bytes())
	return super.NewValue(to, b.Bytes().Body())
}

func upcastUnionTag(types []super.Type, out super.Type) int {
	k := out.Kind()
	if k == super.PrimitiveKind {
		id := out.ID()
		return slices.IndexFunc(types, func(t super.Type) bool { return t.ID() == id })
	}
	return slices.IndexFunc(types, func(t super.Type) bool { return t.Kind() == k })
}

func (u *upcast) toError(from super.Value, to *super.TypeError) super.Value {
	if e, ok := from.Type().(*super.TypeError); ok {
		from = super.NewValue(e.Type, from.Bytes())
	}
	from = u.Cast(from, to.Type)
	if from.Type() != to.Type {
		return from
	}
	return super.NewValue(to, from.Bytes())
}

func (u *upcast) toNamed(from super.Value, to *super.TypeNamed) super.Value {
	from = u.Cast(from, to.Type)
	if from.Type() != to.Type {
		return from
	}
	return super.NewValue(to, from.Bytes())
}

// bestUnionTag tries to return the most specific union tag for in
// within out.  It returns -1 if out is not a union or contains no type
// compatible with in.  (Types are compatible if they have the same underlying
// type.)  If out contains in, BestUnionTag returns its tag.
// Otherwise, if out contains in's underlying type, BestUnionTag returns
// its tag.  Finally, BestUnionTag returns the smallest tag in
// out whose type is compatible with in.
func bestUnionTag(in, out super.Type) int {
	outUnion, ok := super.TypeUnder(out).(*super.TypeUnion)
	if !ok {
		return -1
	}
	typeUnderIn := super.TypeUnder(in)
	underlying := -1
	compatible := -1
	for i, t := range outUnion.Types {
		if t == in {
			return i
		}
		if t == typeUnderIn && underlying == -1 {
			underlying = i
		}
		if super.TypeUnder(t) == typeUnderIn && compatible == -1 {
			compatible = i
		}
	}
	if underlying != -1 {
		return underlying
	}
	return compatible
}
