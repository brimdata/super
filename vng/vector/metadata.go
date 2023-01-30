package vector

import (
	"github.com/brimdata/zed"
	"github.com/brimdata/zed/pkg/field"
)

type Metadata interface {
	Type(*zed.Context) zed.Type
}

type Record struct {
	Fields []Field
}

func (r *Record) Type(zctx *zed.Context) zed.Type {
	fields := make([]zed.Field, 0, len(r.Fields))
	for _, field := range r.Fields {
		typ := field.Values.Type(zctx)
		fields = append(fields, zed.Field{Name: field.Name, Type: typ})
	}
	return zctx.MustLookupTypeRecord(fields)
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
	Lengths []Segment
	Values  Metadata
}

func (a *Array) Type(zctx *zed.Context) zed.Type {
	return zctx.LookupTypeArray(a.Values.Type(zctx))
}

type Set Array

func (s *Set) Type(zctx *zed.Context) zed.Type {
	return zctx.LookupTypeSet(s.Values.Type(zctx))
}

type Map struct {
	Lengths []Segment
	Keys    Metadata
	Values  Metadata
}

func (m *Map) Type(zctx *zed.Context) zed.Type {
	keyType := m.Keys.Type(zctx)
	valType := m.Values.Type(zctx)
	return zctx.LookupTypeMap(keyType, valType)
}

type Union struct {
	Tags   []Segment
	Values []Metadata
}

func (u *Union) Type(zctx *zed.Context) zed.Type {
	types := make([]zed.Type, 0, len(u.Values))
	for _, value := range u.Values {
		types = append(types, value.Type(zctx))
	}
	return zctx.LookupTypeUnion(types)
}

type Named struct {
	Name   string
	Values Metadata
}

func (n *Named) Type(zctx *zed.Context) zed.Type {
	t, err := zctx.LookupTypeNamed(n.Name, n.Values.Type(zctx))
	if err != nil {
		panic(err) //XXX
	}
	return t
}

type Primitive struct {
	Typ    zed.Type `zed:"Type"`
	Segmap []Segment
}

func (p *Primitive) Type(zctx *zed.Context) zed.Type {
	return p.Typ
}

type Nulls struct {
	Runs   []Segment
	Values Metadata
}

func (n *Nulls) Type(zctx *zed.Context) zed.Type {
	return n.Values.Type(zctx)
}

var Template = []interface{}{
	Record{},
	Array{},
	Set{},
	Map{},
	Union{},
	Primitive{},
	Named{},
	Nulls{},
}
