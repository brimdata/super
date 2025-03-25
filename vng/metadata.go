package vng

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/zcode"
)

type Metadata interface {
	Type(*super.Context) super.Type
	Len() uint32
}

type Record struct {
	Length uint32
	Fields []Field
}

func (r *Record) Type(zctx *super.Context) super.Type {
	fields := make([]super.Field, 0, len(r.Fields))
	for _, field := range r.Fields {
		typ := field.Values.Type(zctx)
		fields = append(fields, super.Field{Name: field.Name, Type: typ})
	}
	return zctx.MustLookupTypeRecord(fields)
}

func (r *Record) Len() uint32 {
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

func (r *Record) Lookup(path field.Path) *Field {
	var f *Field
	for _, name := range path {
		f = r.LookupField(name)
		if f == nil {
			return nil
		}
		if next, ok := Under(f.Values).(*Record); ok {
			r = next
		} else {
			break
		}
	}
	return f
}

func Under(meta Metadata) Metadata {
	for {
		switch inner := meta.(type) {
		case *Named:
			meta = inner.Values
		case *Nulls:
			meta = inner.Values
		default:
			return meta
		}
	}
}

type Field struct {
	Name   string
	Values Metadata
}

type Array struct {
	Length  uint32
	Lengths Segment
	Values  Metadata
}

func (a *Array) Type(zctx *super.Context) super.Type {
	return zctx.LookupTypeArray(a.Values.Type(zctx))
}

func (a *Array) Len() uint32 {
	return a.Length
}

type Set Array

func (s *Set) Type(zctx *super.Context) super.Type {
	return zctx.LookupTypeSet(s.Values.Type(zctx))
}

func (s *Set) Len() uint32 {
	return s.Length
}

type Map struct {
	Length  uint32
	Lengths Segment
	Keys    Metadata
	Values  Metadata
}

func (m *Map) Type(zctx *super.Context) super.Type {
	keyType := m.Keys.Type(zctx)
	valType := m.Values.Type(zctx)
	return zctx.LookupTypeMap(keyType, valType)
}

func (m *Map) Len() uint32 {
	return m.Length
}

type Union struct {
	Length uint32
	Tags   Segment
	Values []Metadata
}

func (u *Union) Type(zctx *super.Context) super.Type {
	types := make([]super.Type, 0, len(u.Values))
	for _, value := range u.Values {
		types = append(types, value.Type(zctx))
	}
	return zctx.LookupTypeUnion(types)
}

func (u *Union) Len() uint32 {
	return u.Length
}

type Named struct {
	Name   string
	Values Metadata
}

func (n *Named) Type(zctx *super.Context) super.Type {
	t, err := zctx.LookupTypeNamed(n.Name, n.Values.Type(zctx))
	if err != nil {
		panic(err) //XXX
	}
	return t
}

func (n *Named) Len() uint32 {
	return n.Values.Len()
}

type Error struct {
	Values Metadata
}

func (e *Error) Type(zctx *super.Context) super.Type {
	return zctx.LookupTypeError(e.Values.Type(zctx))
}

func (e *Error) Len() uint32 {
	return e.Values.Len()
}

type Int struct {
	Typ      super.Type `zed:"Type"`
	Location Segment
	Min      int64
	Max      int64
	Count    uint32
}

func (i *Int) Type(*super.Context) super.Type {
	return i.Typ
}

func (i *Int) Len() uint32 {
	return i.Count
}

type Uint struct {
	Typ      super.Type `zed:"Type"`
	Location Segment
	Min      uint64
	Max      uint64
	Count    uint32
}

func (u *Uint) Type(*super.Context) super.Type {
	return u.Typ
}

func (u *Uint) Len() uint32 {
	return u.Count
}

type Primitive struct {
	Typ      super.Type `zed:"Type"`
	Location Segment
	Min      *super.Value
	Max      *super.Value
	Count    uint32
}

func (p *Primitive) Type(zctx *super.Context) super.Type {
	return p.Typ
}

func (p *Primitive) Len() uint32 {
	return p.Count
}

type Nulls struct {
	Runs   Segment
	Values Metadata
	Count  uint32 // Count of nulls
}

func (n *Nulls) Type(zctx *super.Context) super.Type {
	return n.Values.Type(zctx)
}

func (n *Nulls) Len() uint32 {
	return n.Count + n.Values.Len()
}

type Const struct {
	Value super.Value
	Count uint32
}

func (c *Const) Type(zctx *super.Context) super.Type {
	return c.Value.Type()
}

func (c *Const) Len() uint32 {
	return c.Count
}

type Dict struct {
	Values Metadata
	Counts Segment
	Index  Segment
	Length uint32
}

func (d *Dict) Type(zctx *super.Context) super.Type {
	return d.Values.Type(zctx)
}

func (d *Dict) Len() uint32 {
	return d.Length
}

type Dynamic struct {
	Tags   Segment
	Values []Metadata
	Length uint32
}

var _ Metadata = (*Dynamic)(nil)

func (*Dynamic) Type(zctx *super.Context) super.Type {
	panic("Type should not be called on Dynamic")
}

func (d *Dynamic) Len() uint32 {
	return d.Length
}

func MetadataValues(zctx *super.Context, m Metadata) []super.Value {
	var b zcode.Builder
	var values []super.Value
	if dynamic, ok := m.(*Dynamic); ok {
		for _, m := range dynamic.Values {
			b.Reset()
			typ := metadataValue(zctx, &b, m)
			values = append(values, super.NewValue(typ, b.Bytes().Body()))
		}
	} else {
		typ := metadataValue(zctx, &b, m)
		values = append(values, super.NewValue(typ, b.Bytes().Body()))
	}
	return values
}

func metadataValue(zctx *super.Context, b *zcode.Builder, m Metadata) super.Type {
	switch m := Under(m).(type) {
	case *Dict:
		return metadataValue(zctx, b, m.Values)
	case *Record:
		var fields []super.Field
		b.BeginContainer()
		for _, f := range m.Fields {
			fields = append(fields, super.Field{Name: f.Name, Type: metadataValue(zctx, b, f.Values)})
		}
		b.EndContainer()
		return zctx.MustLookupTypeRecord(fields)
	case *Primitive:
		min, max := super.NewValue(m.Typ, nil), super.NewValue(m.Typ, nil)
		if m.Min != nil {
			min = *m.Min
		}
		if m.Max != nil {
			max = *m.Max
		}
		return metadataLeaf(zctx, b, min, max)
	case *Int:
		return metadataLeaf(zctx, b, super.NewInt(m.Typ, m.Min), super.NewInt(m.Typ, m.Max))
	case *Uint:
		return metadataLeaf(zctx, b, super.NewUint(m.Typ, m.Min), super.NewUint(m.Typ, m.Max))
	case *Const:
		return metadataLeaf(zctx, b, m.Value, m.Value)
	default:
		b.Append(nil)
		return super.TypeNull
	}
}

func metadataLeaf(zctx *super.Context, b *zcode.Builder, min, max super.Value) super.Type {
	b.BeginContainer()
	b.Append(min.Bytes())
	b.Append(max.Bytes())
	b.EndContainer()
	return zctx.MustLookupTypeRecord([]super.Field{
		{Name: "min", Type: min.Type()},
		{Name: "max", Type: max.Type()},
	})
}

var Template = []interface{}{
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
