package csup

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/zcode"
)

type Metadata interface {
	Type(*Context, *super.Context) super.Type
	Len(*Context) uint32
}

type Record struct {
	Length uint32
	Fields []Field
}

func (r *Record) Type(cctx *Context, sctx *super.Context) super.Type {
	fields := make([]super.Field, 0, len(r.Fields))
	for _, field := range r.Fields {
		typ := cctx.Lookup(field.Values).Type(cctx, sctx)
		fields = append(fields, super.NewField(field.Name, typ))
	}
	return sctx.MustLookupTypeRecord(fields)
}

func (r *Record) Len(*Context) uint32 {
	return r.Length
}

func (r *Record) LookupField(name string) *Field {
	for k, field := range r.Fields {
		if field.Name == name {
			return &r.Fields[k]
		}
	}
	return nil
}

func under(cctx *Context, meta Metadata) Metadata {
	for {
		switch inner := meta.(type) {
		case *Named:
			meta = cctx.Lookup(inner.Values)
		case *Nulls:
			meta = cctx.Lookup(inner.Values)
		default:
			return meta
		}
	}
}

type Field struct {
	Name   string
	Values ID
}

type Array struct {
	Length  uint32
	Lengths Segment
	Values  ID
}

func (a *Array) Type(cctx *Context, sctx *super.Context) super.Type {
	return sctx.LookupTypeArray(cctx.Lookup(a.Values).Type(cctx, sctx))
}

func (a *Array) Len(*Context) uint32 {
	return a.Length
}

type Set Array

func (s *Set) Type(cctx *Context, sctx *super.Context) super.Type {
	return sctx.LookupTypeSet(cctx.Lookup(s.Values).Type(cctx, sctx))
}

func (s *Set) Len(*Context) uint32 {
	return s.Length
}

type Map struct {
	Length  uint32
	Lengths Segment
	Keys    ID
	Values  ID
}

func (m *Map) Type(cctx *Context, sctx *super.Context) super.Type {
	keyType := cctx.Lookup(m.Keys).Type(cctx, sctx)
	valType := cctx.Lookup(m.Values).Type(cctx, sctx)
	return sctx.LookupTypeMap(keyType, valType)
}

func (m *Map) Len(*Context) uint32 {
	return m.Length
}

type Union struct {
	Length uint32
	Tags   Segment
	Values []ID
}

func (u *Union) Type(cctx *Context, sctx *super.Context) super.Type {
	types := make([]super.Type, 0, len(u.Values))
	for _, value := range u.Values {
		types = append(types, cctx.Lookup(value).Type(cctx, sctx))
	}
	return sctx.LookupTypeUnion(types)
}

func (u *Union) Len(*Context) uint32 {
	return u.Length
}

type Named struct {
	Name   string
	Values ID
}

func (n *Named) Type(cctx *Context, sctx *super.Context) super.Type {
	t, err := sctx.LookupTypeNamed(n.Name, cctx.Lookup(n.Values).Type(cctx, sctx))
	if err != nil {
		panic(err)
	}
	return t
}

func (n *Named) Len(cctx *Context) uint32 {
	return cctx.Lookup(n.Values).Len(cctx)
}

type Error struct {
	Values ID
}

func (e *Error) Type(cctx *Context, sctx *super.Context) super.Type {
	return sctx.LookupTypeError(cctx.Lookup(e.Values).Type(cctx, sctx))
}

func (e *Error) Len(cctx *Context) uint32 {
	return cctx.Lookup(e.Values).Len(cctx)
}

type Int struct {
	Typ      super.Type `super:"Type"`
	Location Segment
	Min      int64
	Max      int64
	Count    uint32
}

func (i *Int) Type(*Context, *super.Context) super.Type {
	return i.Typ
}

func (i *Int) Len(*Context) uint32 {
	return i.Count
}

type Uint struct {
	Typ      super.Type `super:"Type"`
	Location Segment
	Min      uint64
	Max      uint64
	Count    uint32
}

func (u *Uint) Type(*Context, *super.Context) super.Type {
	return u.Typ
}

func (u *Uint) Len(*Context) uint32 {
	return u.Count
}

type Primitive struct {
	Typ      super.Type `super:"Type"`
	Location Segment
	Min      *super.Value
	Max      *super.Value
	Count    uint32
}

func (p *Primitive) Type(*Context, *super.Context) super.Type {
	return p.Typ
}

func (p *Primitive) Len(*Context) uint32 {
	return p.Count
}

type Nulls struct {
	Runs   Segment
	Values ID
	Count  uint32 // Count of nulls
}

func (n *Nulls) Type(cctx *Context, sctx *super.Context) super.Type {
	return cctx.Lookup(n.Values).Type(cctx, sctx)
}

func (n *Nulls) Len(cctx *Context) uint32 {
	return n.Count + cctx.Lookup(n.Values).Len(cctx)
}

type Const struct {
	Value super.Value // this value lives in local context and needs to be translated by shadow
	Count uint32
}

func (c *Const) Type(_ *Context, sctx *super.Context) super.Type {
	typ, err := sctx.TranslateType(c.Value.Type())
	if err != nil {
		panic(err)
	}
	return typ
}

func (c *Const) Len(*Context) uint32 {
	return c.Count
}

type Dict struct {
	Values ID
	Counts Segment
	Index  Segment
	Length uint32
}

func (d *Dict) Type(cctx *Context, sctx *super.Context) super.Type {
	return cctx.Lookup(d.Values).Type(cctx, sctx)
}

func (d *Dict) Len(*Context) uint32 {
	return d.Length
}

type Dynamic struct {
	Tags   Segment
	Values []ID
	Length uint32
}

var _ Metadata = (*Dynamic)(nil)

func (*Dynamic) Type(*Context, *super.Context) super.Type {
	panic("Type should not be called on Dynamic")
}

func (d *Dynamic) Len(*Context) uint32 {
	return d.Length
}

func metadataValue(cctx *Context, sctx *super.Context, b *zcode.Builder, id ID, projection field.Projection) super.Type {
	m := cctx.Lookup(id)
	switch m := under(cctx, m).(type) {
	case *Dict:
		return metadataValue(cctx, sctx, b, m.Values, projection)
	case *Record:
		if len(projection) == 0 {
			var fields []super.Field
			b.BeginContainer()
			for _, f := range m.Fields {
				fields = append(fields, super.NewField(f.Name, metadataValue(cctx, sctx, b, f.Values, nil)))
			}
			b.EndContainer()
			return sctx.MustLookupTypeRecord(fields)
		}
		switch elem := projection[0].(type) {
		case string:
			var fields []super.Field
			// If the field isn't here, we emit an empty record, which will cause
			// the metadata filter expression to properly evaluate the missing
			// value as error missing.
			b.BeginContainer()
			if k := indexOfField(elem, m.Fields); k >= 0 {
				fields = []super.Field{super.NewField(elem, metadataValue(cctx, sctx, b, m.Fields[k].Values, projection[1:]))}
			}
			b.EndContainer()
			return sctx.MustLookupTypeRecord(fields)
		case field.Fork:
			var fields []super.Field
			b.BeginContainer()
			for _, path := range elem {
				if name, ok := path[0].(string); ok {
					if k := indexOfField(name, m.Fields); k >= 0 {
						f := m.Fields[k]
						fields = append(fields, super.NewField(f.Name, metadataValue(cctx, sctx, b, f.Values, projection[1:])))
					}
				}
			}
			b.EndContainer()
			return sctx.MustLookupTypeRecord(fields)
		default:
			panic("bad projection")
		}
	case *Primitive:
		min, max := super.NewValue(m.Typ, nil), super.NewValue(m.Typ, nil)
		if m.Min != nil {
			min = *m.Min
		}
		if m.Max != nil {
			max = *m.Max
		}
		return metadataLeaf(sctx, b, min, max)
	case *Int:
		return metadataLeaf(sctx, b, super.NewInt(m.Typ, m.Min), super.NewInt(m.Typ, m.Max))
	case *Uint:
		return metadataLeaf(sctx, b, super.NewUint(m.Typ, m.Min), super.NewUint(m.Typ, m.Max))
	case *Const:
		return metadataLeaf(sctx, b, m.Value, m.Value)
	default:
		b.Append(nil)
		return super.TypeNull
	}
}

func metadataLeaf(sctx *super.Context, b *zcode.Builder, min, max super.Value) super.Type {
	b.BeginContainer()
	b.Append(min.Bytes())
	b.Append(max.Bytes())
	b.EndContainer()
	return sctx.MustLookupTypeRecord([]super.Field{
		{Name: "min", Type: min.Type()},
		{Name: "max", Type: max.Type()},
	})
}

func indexOfField(name string, fields []Field) int {
	return slices.IndexFunc(fields, func(f Field) bool {
		return f.Name == name
	})
}

var Template = []any{
	Record{},
	Array{},
	Set{},
	Map{},
	Union{},
	Int{},
	Uint{},
	Primitive{},
	Named{},
	Error{},
	Nulls{},
	Const{},
	Dict{},
	Dynamic{},
}
