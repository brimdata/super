package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type unblend struct {
	sctx *super.Context
}

func NewUnblend(sctx *super.Context) *unblend {
	return &unblend{sctx}
}

func (u *unblend) Call(args []super.Value) super.Value {
	return u.eval(args[0])
}

func (u *unblend) eval(in super.Value) super.Value {
	switch typ := in.Type().(type) {
	case *super.TypeRecord:
		var fields []super.Field
		var elems []super.Value
		it := scode.NewRecordIter(in.Bytes(), typ.Opts)
		for _, f := range typ.Fields {
			bytes, none := it.Next(f.Opt)
			if none {
				continue
			}
			val := u.eval(super.NewValue(f.Type, bytes))
			elems = append(elems, val)
			fields = append(fields, super.NewField(f.Name, val.Type()))
		}
		var b scode.Builder
		for _, e := range elems {
			b.Append(e.Bytes())
		}
		return super.NewValue(u.sctx.MustLookupTypeRecord(fields), b.Bytes())
	case *super.TypeArray:
		elems := u.arrayOrSet(typ.Type, in.Bytes())
		if len(elems) == 0 {
			typ := u.sctx.LookupTypeArray(super.TypeNull)
			return super.NewValue(typ, nil)
		}
		elemType, bytes := u.unify(elems)
		return super.NewValue(u.sctx.LookupTypeArray(elemType), bytes)
	case *super.TypeSet:
		elems := u.arrayOrSet(typ.Type, in.Bytes())
		if len(elems) == 0 {
			typ := u.sctx.LookupTypeSet(super.TypeNull)
			return super.NewValue(typ, nil)
		}
		elemType, bytes := u.unify(elems)
		return super.NewValue(u.sctx.LookupTypeSet(elemType), bytes)
	case *super.TypeMap:
		var keys, vals []super.Value
		for it := in.Bytes().Iter(); !it.Done(); {
			keys = append(keys, super.NewValue(typ.KeyType, it.Next()).Deunion())
			vals = append(vals, super.NewValue(typ.ValType, it.Next()).Deunion())
		}
		keyType := u.unifyType(keys)
		valType := u.unifyType(vals)
		var b scode.Builder
		for k, key := range keys {
			if u, ok := keyType.(*super.TypeUnion); ok {
				super.BuildUnion(&b, u.TagOf(key.Type()), key.Bytes())
			} else {
				b.Append(key.Bytes())
			}
			val := vals[k]
			if u, ok := valType.(*super.TypeUnion); ok {
				super.BuildUnion(&b, u.TagOf(val.Type()), val.Bytes())
			} else {
				b.Append(val.Bytes())
			}
		}
		return super.NewValue(u.sctx.LookupTypeMap(keyType, valType), b.Bytes())
	case *super.TypeUnion:
		return u.eval(in.Deunion())
	default:
		// primitives, named types, enums
		return in
	}
}

func (u *unblend) arrayOrSet(typ super.Type, bytes scode.Bytes) []super.Value {
	var elems []super.Value
	for it := bytes.Iter(); !it.Done(); {
		elems = append(elems, u.eval(super.NewValue(typ, it.Next())))
	}
	return elems
}

func (u *unblend) unify(elems []super.Value) (super.Type, scode.Bytes) {
	seen := make(map[super.Type]struct{})
	var types []super.Type
	for _, e := range elems {
		typ := e.Type()
		if _, ok := seen[typ]; !ok {
			seen[typ] = struct{}{}
			types = append(types, typ)
		}
	}
	if len(types) == 1 {
		var b scode.Builder
		for _, e := range elems {
			b.Append(e.Bytes())
		}
		return types[0], b.Bytes()
	}
	var b scode.Builder
	union := u.sctx.LookupTypeUnion(types)
	for _, e := range elems {
		super.BuildUnion(&b, union.TagOf(e.Type()), e.Bytes())
	}
	return union, b.Bytes()
}

func (u *unblend) unifyType(vals []super.Value) super.Type {
	seen := make(map[super.Type]struct{})
	var types []super.Type
	for _, e := range vals {
		typ := e.Type()
		if _, ok := seen[typ]; !ok {
			seen[typ] = struct{}{}
			types = append(types, typ)
		}
	}
	switch len(types) {
	case 0:
		return super.TypeNull // XXX should be TypeNone
	case 1:
		return types[0]
	default:
		return u.sctx.LookupTypeUnion(types)
	}
}
