package csup

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
)

//XXX get id of Type method in Metadata?  Maybe compute types from shadow only?

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
		fields = append(fields, super.Field{Name: field.Name, Type: typ})
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

// XXX what's this for?  we shouldn't be doing lookup by path here.
// any lookups should be done on the shadow where everything
// has been deserialized by path already
func (r *Record) Lookup(cctx *Context, path field.Path) *Field {
	var f *Field
	for _, name := range path {
		f = r.LookupField(name)
		if f == nil {
			return nil
		}
		if next, ok := Under(cctx, cctx.Lookup(f.Values)).(*Record); ok {
			r = next
		} else {
			break
		}
	}
	return f
}

func Under(cctx *Context, meta Metadata) Metadata {
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
	Typ      super.Type `zed:"Type"`
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
	Typ      super.Type `zed:"Type"`
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
	Typ      super.Type `zed:"Type"`
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

/*
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
*/

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
