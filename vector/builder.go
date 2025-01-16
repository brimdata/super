package vector

import (
	"fmt"
	"net/netip"

	"github.com/RoaringBitmap/roaring"
	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
)

type Builder interface {
	Write(zcode.Bytes)
	Build() Any
}

type DynamicBuilder struct {
	tags   []uint32
	values []Builder
	which  map[super.Type]int
}

func NewDynamicBuilder() *DynamicBuilder {
	return &DynamicBuilder{
		which: make(map[super.Type]int),
	}
}

func (d *DynamicBuilder) Write(val super.Value) {
	typ := val.Type()
	tag, ok := d.which[typ]
	if !ok {
		tag = len(d.values)
		d.values = append(d.values, NewBuilder(typ))
		d.which[typ] = tag
	}
	d.tags = append(d.tags, uint32(tag))
	d.values[tag].Write(val.Bytes())
}

func (d *DynamicBuilder) Build() Any {
	var vecs []Any
	for _, b := range d.values {
		vecs = append(vecs, b.Build())
	}
	if len(vecs) == 1 {
		return vecs[0]
	}
	return NewDynamic(d.tags, vecs)
}

func NewBuilder(typ super.Type) Builder {
	var b Builder
	switch typ := typ.(type) {
	case *super.TypeNamed:
		return &namedBuilder{typ: typ, Builder: NewBuilder(typ.Type)}
	case *super.TypeRecord:
		b = newRecordBuilder(typ)
	case *super.TypeError:
		b = &errorBuilder{typ: typ, Builder: NewBuilder(typ.Type)}
	case *super.TypeArray:
		b = newArraySetBuilder(typ)
	case *super.TypeSet:
		b = newArraySetBuilder(typ)
	case *super.TypeMap:
		b = newMapBuilder(typ)
	case *super.TypeUnion:
		b = newUnionBuilder(typ)
	default:
		id := typ.ID()
		if super.IsNumber(id) {
			switch {
			case super.IsUnsigned(id):
				b = &uintBuilder{typ: typ}
			case super.IsSigned(id):
				b = &intBuilder{typ: typ}
			case super.IsFloat(id):
				b = &floatBuilder{typ: typ}
			}
		} else {
			switch id {
			case super.IDBool:
				b = newBoolBuilder()
			case super.IDBytes, super.IDString, super.IDType:
				b = newBytesStringTypeBuilder(typ)
			case super.IDIP:
				b = &ipBuilder{}
			case super.IDNet:
				b = &netBuilder{}
			case super.IDNull:
				return &constNullBuilder{}
			default:
				panic(fmt.Sprintf("unsupported type: %T", typ))
			}
		}
	}
	return newNullsBuilder(b)
}

type nullsBuilder struct {
	n      uint32
	values Builder
	nulls  *roaring.Bitmap
}

func newNullsBuilder(values Builder) Builder {
	return &nullsBuilder{
		values: values,
		nulls:  roaring.New(),
	}
}

func (n *nullsBuilder) Write(bytes zcode.Bytes) {
	if bytes == nil {
		n.nulls.Add(n.n)
	}
	n.values.Write(bytes)
	n.n++
}

type namedBuilder struct {
	Builder
	typ *super.TypeNamed
}

func (n *namedBuilder) Build() Any {
	return NewNamed(n.typ, n.Builder.Build())
}

func (n *nullsBuilder) Build() Any {
	vec := n.values.Build()
	if !n.nulls.IsEmpty() {
		bits := make([]uint64, (n.n+63)/64)
		n.nulls.WriteDenseTo(bits)
		vec = CopyAndSetNulls(vec, NewBool(bits, n.n, nil))
	}
	return vec
}

type recordBuilder struct {
	typ    *super.TypeRecord
	values []Builder
	len    uint32
}

func newRecordBuilder(typ *super.TypeRecord) Builder {
	var values []Builder
	for _, f := range typ.Fields {
		values = append(values, NewBuilder(f.Type))
	}
	return &recordBuilder{typ: typ, values: values}
}

func (r *recordBuilder) Write(bytes zcode.Bytes) {
	r.len++
	if bytes == nil {
		for _, v := range r.values {
			v.Write(nil)
		}
		return
	}
	it := bytes.Iter()
	for _, v := range r.values {
		v.Write(it.Next())
	}
}

func (r *recordBuilder) Build() Any {
	var vecs []Any
	for _, v := range r.values {
		vecs = append(vecs, v.Build())
	}
	return NewRecord(r.typ, vecs, r.len, nil)
}

type errorBuilder struct {
	typ *super.TypeError
	Builder
}

func (e *errorBuilder) Build() Any {
	return NewError(e.typ, e.Builder.Build(), nil)
}

type arraySetBuilder struct {
	typ     super.Type
	values  Builder
	offsets []uint32
}

func newArraySetBuilder(typ super.Type) Builder {
	return &arraySetBuilder{typ: typ, values: NewBuilder(super.InnerType(typ)), offsets: []uint32{0}}
}

func (a *arraySetBuilder) Write(bytes zcode.Bytes) {
	off := a.offsets[len(a.offsets)-1]
	for it := bytes.Iter(); !it.Done(); {
		a.values.Write(it.Next())
		off++
	}
	a.offsets = append(a.offsets, off)
}

func (a *arraySetBuilder) Build() Any {
	if typ, ok := a.typ.(*super.TypeArray); ok {
		return NewArray(typ, a.offsets, a.values.Build(), nil)
	}
	return NewSet(a.typ.(*super.TypeSet), a.offsets, a.values.Build(), nil)
}

type mapBuilder struct {
	typ          *super.TypeMap
	keys, values Builder
	offsets      []uint32
}

func newMapBuilder(typ *super.TypeMap) Builder {
	return &mapBuilder{
		typ:     typ,
		keys:    NewBuilder(typ.KeyType),
		values:  NewBuilder(typ.ValType),
		offsets: []uint32{0},
	}
}

func (m *mapBuilder) Write(bytes zcode.Bytes) {
	off := m.offsets[len(m.offsets)-1]
	it := bytes.Iter()
	for !it.Done() {
		m.keys.Write(it.Next())
		m.values.Write(it.Next())
		off++
	}
	m.offsets = append(m.offsets, off)
}

func (m *mapBuilder) Build() Any {
	return NewMap(m.typ, m.offsets, m.keys.Build(), m.values.Build(), nil)
}

type unionBuilder struct {
	typ    *super.TypeUnion
	values []Builder
	tags   []uint32
}

func newUnionBuilder(typ *super.TypeUnion) Builder {
	var values []Builder
	for _, typ := range typ.Types {
		values = append(values, NewBuilder(typ))
	}
	return &unionBuilder{typ: typ, values: values}
}

func (u *unionBuilder) Write(bytes zcode.Bytes) {
	if bytes == nil {
		u.tags = append(u.tags, 0)
		return
	}
	var typ super.Type
	typ, bytes = u.typ.Untag(bytes)
	tag := u.typ.TagOf(typ)
	u.values[tag].Write(bytes)
	u.tags = append(u.tags, uint32(tag))
}

func (u *unionBuilder) Build() Any {
	var vecs []Any
	for _, v := range u.values {
		vecs = append(vecs, v.Build())
	}
	return NewUnion(u.typ, u.tags, vecs, nil)
}

type intBuilder struct {
	typ    super.Type
	values []int64
}

func (i *intBuilder) Write(bytes zcode.Bytes) {
	i.values = append(i.values, super.DecodeInt(bytes))
}

func (i *intBuilder) Build() Any {
	return NewInt(i.typ, i.values, nil)
}

type uintBuilder struct {
	typ    super.Type
	values []uint64
}

func (u *uintBuilder) Write(bytes zcode.Bytes) {
	u.values = append(u.values, super.DecodeUint(bytes))
}

func (u *uintBuilder) Build() Any {
	return NewUint(u.typ, u.values, nil)
}

type floatBuilder struct {
	typ    super.Type
	values []float64
}

func (f *floatBuilder) Write(bytes zcode.Bytes) {
	f.values = append(f.values, super.DecodeFloat(bytes))
}

func (f *floatBuilder) Build() Any {
	return NewFloat(f.typ, f.values, nil)
}

type boolBuilder struct {
	values *roaring.Bitmap
	n      uint32
}

func newBoolBuilder() Builder {
	return &boolBuilder{values: roaring.New()}
}

func (b *boolBuilder) Write(bytes zcode.Bytes) {
	if super.DecodeBool(bytes) {
		b.values.Add(b.n)
	}
	b.n++
}

func (b *boolBuilder) Build() Any {
	bits := make([]uint64, (b.n+63)/64)
	b.values.WriteDenseTo(bits)
	return NewBool(bits, b.n, nil)
}

type bytesStringTypeBuilder struct {
	typ   super.Type
	offs  []uint32
	bytes []byte
}

func newBytesStringTypeBuilder(typ super.Type) Builder {
	return &bytesStringTypeBuilder{typ: typ, offs: []uint32{0}}
}

func (b *bytesStringTypeBuilder) Write(bytes zcode.Bytes) {
	b.bytes = append(b.bytes, bytes...)
	b.offs = append(b.offs, uint32(len(b.bytes)))
}

func (b *bytesStringTypeBuilder) Build() Any {
	switch b.typ.ID() {
	case super.IDString:
		return NewString(b.offs, b.bytes, nil)
	case super.IDBytes:
		return NewBytes(b.offs, b.bytes, nil)
	default:
		return NewTypeValue(b.offs, b.bytes, nil)
	}
}

type ipBuilder struct {
	values []netip.Addr
}

func (i *ipBuilder) Write(bytes zcode.Bytes) {
	i.values = append(i.values, super.DecodeIP(bytes))
}

func (i *ipBuilder) Build() Any {
	return NewIP(i.values, nil)
}

type netBuilder struct {
	values []netip.Prefix
}

func (n *netBuilder) Write(bytes zcode.Bytes) {
	n.values = append(n.values, super.DecodeNet(bytes))
}

func (n *netBuilder) Build() Any {
	return NewNet(n.values, nil)
}

type constNullBuilder struct {
	n uint32
}

func (c *constNullBuilder) Write(bytes zcode.Bytes) {
	c.n++
}

func (c *constNullBuilder) Build() Any {
	return NewConst(super.Null, c.n, nil)
}
