package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/scode"
)

type Flatten struct {
	scode.Builder
	keyType    super.Type
	entryTypes map[super.Type]super.Type
	sctx       *super.Context

	// This exists only to reduce memory allocations.
	types []super.Type
}

func NewFlatten(sctx *super.Context) *Flatten {
	return &Flatten{
		entryTypes: make(map[super.Type]super.Type),
		keyType:    sctx.LookupTypeArray(super.TypeString),
		sctx:       sctx,
	}
}

func (n *Flatten) Call(args []super.Value) super.Value {
	val := args[0]
	typ := super.TypeRecordOf(val.Type())
	if typ == nil {
		return val
	}
	inner := n.innerTypeOf(typ.Fields)
	n.Reset()
	n.encode(typ.Fields, inner, field.Path{}, val.Bytes())
	return super.NewValue(n.sctx.LookupTypeArray(inner), n.Bytes())
}

func (n *Flatten) innerTypeOf(fields []super.Field) super.Type {
	n.types = n.appendTypes(n.types[:0], fields)
	unique := super.UniqueTypes(n.types)
	if len(unique) == 1 {
		return unique[0]
	}
	return n.sctx.LookupTypeUnion(unique)
}

func (n *Flatten) appendTypes(types []super.Type, fields []super.Field) []super.Type {
	for _, f := range fields {
		if typ := super.TypeRecordOf(f.Type); typ != nil {
			types = n.appendTypes(types, typ.Fields)
			continue
		}
		typ, ok := n.entryTypes[f.Type]
		if !ok {
			typ = n.sctx.MustLookupTypeRecord([]super.Field{
				super.NewField("key", n.keyType),
				super.NewField("value", f.Type),
			})
			n.entryTypes[f.Type] = typ
		}
		types = append(types, typ)
	}
	return types
}

func (n *Flatten) encode(fields []super.Field, inner super.Type, base field.Path, b scode.Bytes) {
	it := b.Iter()
	for _, f := range fields {
		val := it.Next()
		key := append(base, f.Name)
		if typ := super.TypeRecordOf(f.Type); typ != nil {
			n.encode(typ.Fields, inner, key, val)
			continue
		}
		typ := n.entryTypes[f.Type]
		union, ok := inner.(*super.TypeUnion)
		if ok {
			super.BeginUnion(&n.Builder, union.TagOf(typ))
		}
		n.BeginContainer()
		n.encodeKey(key)
		n.Append(val)
		n.EndContainer()
		if ok {
			n.EndContainer()
		}
	}
}

func (n *Flatten) encodeKey(key field.Path) {
	n.BeginContainer()
	for _, name := range key {
		n.Append(super.EncodeString(name))
	}
	n.EndContainer()
}
