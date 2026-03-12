package function

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type defuse struct {
	sctx     *super.Context
	downcast *downcast
	has      map[super.Type]bool
}

func NewDefuse(sctx *super.Context) *defuse {
	_, d := newDowncastDefuser(sctx)
	return d
}

func newDowncastDefuser(sctx *super.Context) (*downcast, *defuse) {
	d := &defuse{
		sctx:     sctx,
		downcast: &downcast{sctx: sctx},
		has:      make(map[super.Type]bool),
	}
	d.downcast.defuser = d
	return d.downcast, d
}

func (d *defuse) Call(args []super.Value) super.Value {
	return d.eval(args[0])
}

func (d *defuse) eval(in super.Value) super.Value {
	if !d.hasFusion(in.Type()) {
		return in
	}
	switch typ := in.Type().(type) {
	case *super.TypeRecord:
		var fields []super.Field
		var b scode.Builder
		var optOff int
		var nones []int
		b.BeginContainer()
		it := scode.NewRecordIter(in.Bytes(), typ.Opts)
		for _, f := range typ.Fields {
			bytes, none := it.Next(f.Opt)
			if none {
				nones = append(nones, optOff)
				optOff++
				continue
			}
			val := d.eval(super.NewValue(f.Type, bytes))
			b.Append(val.Bytes())
			fields = append(fields, super.NewField(f.Name, val.Type()))
		}
		b.EndContainerWithNones(typ.Opts, nones)
		return super.NewValue(d.sctx.MustLookupTypeRecord(fields), b.Bytes())
	case *super.TypeArray:
		elems := d.parseArrayOrSet(typ.Type, in.Bytes())
		if len(elems) == 0 {
			typ := d.sctx.LookupTypeArray(super.TypeNull)
			return super.NewValue(typ, nil)
		}
		elemType, bytes := d.unify(elems)
		return super.NewValue(d.sctx.LookupTypeArray(elemType), bytes)
	case *super.TypeSet:
		elems := d.parseArrayOrSet(typ.Type, in.Bytes())
		if len(elems) == 0 {
			typ := d.sctx.LookupTypeArray(super.TypeNull)
			return super.NewValue(typ, nil)
		}
		elemType, bytes := d.unify(elems)
		return super.NewValue(d.sctx.LookupTypeSet(elemType), bytes)
	case *super.TypeMap:
		var keys, vals []super.Value
		for it := in.Bytes().Iter(); !it.Done(); {
			keys = append(keys, super.NewValue(typ, it.Next()))
			vals = append(vals, super.NewValue(typ, it.Next()))
		}
		keyType := d.unifyType(keys)
		valType := d.unifyType(vals)
		var b scode.Builder
		for k, key := range keys {
			if u, ok := keyType.(*super.TypeUnion); ok {
				super.BuildUnion(&b, u.TagOf(u), key.Bytes())
			} else {
				b.Append(key.Bytes())
			}
			val := vals[k]
			if u, ok := valType.(*super.TypeUnion); ok {
				super.BuildUnion(&b, u.TagOf(u), val.Bytes())
			} else {
				b.Append(val.Bytes())
			}
		}
		return super.NewValue(d.sctx.LookupTypeMap(keyType, valType), b.Bytes())
	case *super.TypeUnion:
		return d.eval(in.Deunion())
	case *super.TypeFusion:
		_, subType := typ.Deref(d.sctx, in.Bytes())
		if out, ok := d.downcast.Cast(in, subType); ok {
			return out
		}
		return d.sctx.WrapError("cannot defuse super value", in)
	default:
		// primitives, named types, enums
		// BTW, named types are a barrier to defuse.
		return in
	}
}

func (d *defuse) parseArrayOrSet(typ super.Type, bytes scode.Bytes) []super.Value {
	var elems []super.Value
	for it := bytes.Iter(); !it.Done(); {
		elems = append(elems, d.eval(super.NewValue(typ, it.Next())))
	}
	return elems
}

func (d *defuse) unify(elems []super.Value) (super.Type, scode.Bytes) {
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
	union := d.sctx.LookupTypeUnion(types)
	for _, e := range elems {
		super.BuildUnion(&b, union.TagOf(e.Type()), e.Bytes())
	}
	return union, b.Bytes()
}

func (d *defuse) unifyType(vals []super.Value) super.Type {
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
		return d.sctx.LookupTypeUnion(types)
	}
}

func (d *defuse) hasFusion(typ super.Type) bool {
	if fused, ok := d.has[typ]; ok {
		return fused
	}
	var has bool
	switch typ := typ.(type) {
	case *super.TypeRecord:
		has = slices.ContainsFunc(typ.Fields, func(f super.Field) bool { return d.hasFusion(f.Type) })
	case *super.TypeArray:
		has = d.hasFusion(typ.Type)
	case *super.TypeSet:
		has = d.hasFusion(typ.Type)
	case *super.TypeMap:
		has = d.hasFusion(typ.KeyType) || d.hasFusion(typ.ValType)
	case *super.TypeError:
		has = d.hasFusion(typ.Type)
	case *super.TypeFusion:
		has = true
	}
	d.has[typ] = has
	return has
}
