package vector

import (
	"net/netip"

	"github.com/RoaringBitmap/roaring/v2"
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector/bitvec"
)

type Builder interface {
	Write(scode.Bytes)
	Build(sctx *super.Context) Any
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

func (d *DynamicBuilder) Build(sctx *super.Context) Any {
	var vecs []Any
	for _, b := range d.values {
		vecs = append(vecs, b.Build(sctx))
	}
	if len(vecs) == 1 {
		return vecs[0]
	}
	return NewDynamic(d.tags, vecs)
}

func NewBuilder(typ super.Type) Builder {
	switch typ := typ.(type) {
	case *super.TypeOfUint8,
		*super.TypeOfUint16,
		*super.TypeOfUint32,
		*super.TypeOfUint64:
		return &uintBuilder{typ: typ}
	case *super.TypeOfInt8,
		*super.TypeOfInt16,
		*super.TypeOfInt32,
		*super.TypeOfInt64,
		*super.TypeOfDuration,
		*super.TypeOfTime:
		return &intBuilder{typ: typ}
	case *super.TypeOfFloat16,
		*super.TypeOfFloat32,
		*super.TypeOfFloat64:
		return &floatBuilder{typ: typ}
	case *super.TypeOfBool:
		return newBoolBuilder()
	case *super.TypeOfBytes,
		*super.TypeOfString:
		return newBytesStringTypeBuilder(typ)
	case *super.TypeOfIP:
		return &ipBuilder{}
	case *super.TypeOfNet:
		return &netBuilder{}
	case *super.TypeOfType:
		return newBytesStringTypeBuilder(typ)
	case *super.TypeOfNull:
		return &nullBuilder{}
	case *super.TypeRecord:
		return newRecordBuilder(typ)
	case *super.TypeArray:
		return newArraySetBuilder(typ)
	case *super.TypeSet:
		return newArraySetBuilder(typ)
	case *super.TypeMap:
		return newMapBuilder(typ)
	case *super.TypeUnion:
		return newUnionBuilder(typ)
	case *super.TypeFusion:
		return newFusionBuilder(typ)
	case *super.TypeEnum:
		return &enumBuilder{typ, nil}
	case *super.TypeError:
		return &errorBuilder{typ: typ, Builder: NewBuilder(typ.Type)}
	case *super.TypeNamed:
		return &namedBuilder{typ: typ, Builder: NewBuilder(typ.Type)}
	}
	panic(typ)
}

type namedBuilder struct {
	Builder
	typ *super.TypeNamed
}

func (n *namedBuilder) Build(sctx *super.Context) Any {
	return NewNamed(n.typ, n.Builder.Build(sctx))
}

type recordBuilder struct {
	typ    *super.TypeRecord
	fields []fieldBuilder
	len    uint32
}

func newRecordBuilder(typ *super.TypeRecord) Builder {
	var fields []fieldBuilder
	for _, f := range typ.Fields {
		fields = append(fields, fieldBuilder{opt: f.Opt, val: NewBuilder(f.Type)})
	}
	return &recordBuilder{typ: typ, fields: fields}
}

func (r *recordBuilder) Write(bytes scode.Bytes) {
	off := r.len
	r.len++
	it := scode.NewRecordIter(bytes, r.typ.Opts)
	for k := range r.fields {
		elem, none := it.Next(r.typ.Fields[k].Opt)
		// The none condition is captured by RLE.
		if !none {
			r.fields[k].write(elem, off)
		}
	}
}

func (r *recordBuilder) Build(sctx *super.Context) Any {
	var fields []Any
	for k := range r.fields {
		fields = append(fields, r.fields[k].build(sctx, r.len))
	}
	return NewRecord(r.typ, fields, r.len)
}

type fieldBuilder struct {
	opt  bool
	val  Builder
	runs RLE
}

func (f *fieldBuilder) write(bytes scode.Bytes, off uint32) {
	if f.opt {
		f.runs.Touch(off)
	}
	f.val.Write(bytes)
}

func (f *fieldBuilder) build(sctx *super.Context, n uint32) Any {
	var runs []uint32
	if f.opt {
		runs = f.runs.End(n)
	}
	return NewFieldFromRLE(sctx, f.val.Build(sctx), n, runs)
}

type errorBuilder struct {
	typ *super.TypeError
	Builder
}

func (e *errorBuilder) Build(sctx *super.Context) Any {
	return NewError(e.typ, e.Builder.Build(sctx))
}

type arraySetBuilder struct {
	typ     super.Type
	values  Builder
	offsets []uint32
}

func newArraySetBuilder(typ super.Type) Builder {
	return &arraySetBuilder{typ: typ, values: NewBuilder(super.InnerType(typ)), offsets: []uint32{0}}
}

func (a *arraySetBuilder) Write(bytes scode.Bytes) {
	off := a.offsets[len(a.offsets)-1]
	for it := bytes.Iter(); !it.Done(); {
		a.values.Write(it.Next())
		off++
	}
	a.offsets = append(a.offsets, off)
}

func (a *arraySetBuilder) Build(sctx *super.Context) Any {
	if typ, ok := a.typ.(*super.TypeArray); ok {
		return NewArray(typ, a.offsets, a.values.Build(sctx))
	}
	return NewSet(a.typ.(*super.TypeSet), a.offsets, a.values.Build(sctx))
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

func (m *mapBuilder) Write(bytes scode.Bytes) {
	off := m.offsets[len(m.offsets)-1]
	it := bytes.Iter()
	for !it.Done() {
		m.keys.Write(it.Next())
		m.values.Write(it.Next())
		off++
	}
	m.offsets = append(m.offsets, off)
}

func (m *mapBuilder) Build(sctx *super.Context) Any {
	return NewMap(m.typ, m.offsets, m.keys.Build(sctx), m.values.Build(sctx))
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

func (u *unionBuilder) Write(bytes scode.Bytes) {
	var typ super.Type
	typ, bytes = u.typ.Untag(bytes)
	tag := u.typ.TagOf(typ)
	u.values[tag].Write(bytes)
	u.tags = append(u.tags, uint32(tag))
}

func (u *unionBuilder) Build(sctx *super.Context) Any {
	var vecs []Any
	for _, v := range u.values {
		vecs = append(vecs, v.Build(sctx))
	}
	return NewUnion(u.typ, u.tags, vecs)
}

type fusionBuilder struct {
	typ    *super.TypeFusion
	values Builder
	//XXX this will change
	subTypes Builder
}

func newFusionBuilder(typ *super.TypeFusion) Builder {
	return &fusionBuilder{typ: typ, values: NewBuilder(typ.Type), subTypes: newBytesStringTypeBuilder(super.TypeType)}
}

func (f *fusionBuilder) Write(bytes scode.Bytes) {
	it := bytes.Iter()
	f.values.Write(it.Next())
	f.subTypes.Write(it.Next())
}

func (f *fusionBuilder) Build(sctx *super.Context) Any {
	return NewFusion(f.typ, f.values.Build(sctx), f.subTypes.Build(sctx).(*TypeValue))
}

type enumBuilder struct {
	typ    *super.TypeEnum
	values []uint64
}

func (e *enumBuilder) Write(bytes scode.Bytes) {
	e.values = append(e.values, super.DecodeUint(bytes))
}

func (e *enumBuilder) Build(sctx *super.Context) Any {
	return NewEnum(e.typ, e.values)
}

type intBuilder struct {
	typ    super.Type
	values []int64
}

func (i *intBuilder) Write(bytes scode.Bytes) {
	i.values = append(i.values, super.DecodeInt(bytes))
}

func (i *intBuilder) Build(sctx *super.Context) Any {
	return NewInt(i.typ, i.values)
}

type uintBuilder struct {
	typ    super.Type
	values []uint64
}

func (u *uintBuilder) Write(bytes scode.Bytes) {
	u.values = append(u.values, super.DecodeUint(bytes))
}

func (u *uintBuilder) Build(sctx *super.Context) Any {
	return NewUint(u.typ, u.values)
}

type floatBuilder struct {
	typ    super.Type
	values []float64
}

func (f *floatBuilder) Write(bytes scode.Bytes) {
	f.values = append(f.values, super.DecodeFloat(bytes))
}

func (f *floatBuilder) Build(sctx *super.Context) Any {
	return NewFloat(f.typ, f.values)
}

type boolBuilder struct {
	values *roaring.Bitmap
	n      uint32
}

func newBoolBuilder() Builder {
	return &boolBuilder{values: roaring.New()}
}

func (b *boolBuilder) Write(bytes scode.Bytes) {
	if super.DecodeBool(bytes) {
		b.values.Add(b.n)
	}
	b.n++
}

func (b *boolBuilder) Build(sctx *super.Context) Any {
	bits := make([]uint64, (b.n+63)/64)
	b.values.WriteDenseTo(bits)
	return NewBool(bitvec.New(bits, b.n))
}

type bytesStringTypeBuilder struct {
	typ   super.Type
	offs  []uint32
	bytes []byte
}

func newBytesStringTypeBuilder(typ super.Type) Builder {
	return &bytesStringTypeBuilder{typ: typ, bytes: []byte{}, offs: []uint32{0}}
}

func (b *bytesStringTypeBuilder) Write(bytes scode.Bytes) {
	b.bytes = append(b.bytes, bytes...)
	b.offs = append(b.offs, uint32(len(b.bytes)))
}

func (b *bytesStringTypeBuilder) Build(sctx *super.Context) Any {
	switch b.typ.ID() {
	case super.IDString:
		return NewString(NewBytesTable(b.offs, b.bytes))
	case super.IDBytes:
		return NewBytes(NewBytesTable(b.offs, b.bytes))
	default:
		return NewTypeValue(NewBytesTable(b.offs, b.bytes))
	}
}

type ipBuilder struct {
	values []netip.Addr
}

func (i *ipBuilder) Write(bytes scode.Bytes) {
	i.values = append(i.values, super.DecodeIP(bytes))
}

func (i *ipBuilder) Build(sctx *super.Context) Any {
	return NewIP(i.values)
}

type netBuilder struct {
	values []netip.Prefix
}

func (n *netBuilder) Write(bytes scode.Bytes) {
	n.values = append(n.values, super.DecodeNet(bytes))
}

func (n *netBuilder) Build(sctx *super.Context) Any {
	return NewNet(n.values)
}

type nullBuilder struct {
	n uint32
}

func (c *nullBuilder) Write(scode.Bytes) {
	c.n++
}

func (c *nullBuilder) Build(*super.Context) Any {
	return NewNull(c.n)
}
