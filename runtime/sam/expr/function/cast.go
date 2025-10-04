package function

import (
	"maps"
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/sup"
)

type cast struct {
	sctx *super.Context
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
		return c.cast(from, typ)
	}
	return c.sctx.WrapError("cast target must be a type or type name", to)
}

func (c *cast) cast(from super.Value, to super.Type) super.Value {
	if from.IsNull() {
		return super.NewValue(to, nil)
	}
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
		if from.IsNull() {
			return super.NewValue(to, nil)
		}
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
			val2 = c.cast(*fieldVal, f.Type)
		} else {
			val2 = super.NewValue(f.Type, nil)
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
	return c.cast(val, to)
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
	tag := expr.BestUnionTag(from.Type(), to)
	if tag < 0 {
		from2 := from.Deunion()
		tag = expr.BestUnionTag(from2.Type(), to)
		if tag < 0 {
			return c.error(from, to)
		}
		from = from2
	}
	bytes := from.Bytes()
	if bytes != nil {
		bytes = scode.Append(scode.Append(nil, super.EncodeInt(int64(tag))), bytes)
	}
	return super.NewValue(to, bytes)
}

func (c *cast) toError(from super.Value, to *super.TypeError) super.Value {
	from = c.cast(from, to.Type)
	if from.Type() != to.Type {
		return from
	}
	return super.NewValue(to, from.Bytes())
}

func (c *cast) toNamed(from super.Value, to *super.TypeNamed) super.Value {
	from = c.cast(from, to.Type)
	if from.Type() != to.Type {
		return from
	}
	return super.NewValue(to, from.Bytes())
}
