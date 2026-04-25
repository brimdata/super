package vector

import (
	"net/netip"

	"github.com/RoaringBitmap/roaring/v2"
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector/bitvec"
)

type ValueBuilder interface {
	Write(scode.Bytes)
	Build(sctx *super.Context) Any
}

type DynamicValueBuilder struct {
	tags   []uint32
	values []ValueBuilder
	which  map[super.Type]int
}

func NewDynamicValueBuilder() *DynamicValueBuilder {
	return &DynamicValueBuilder{
		which: make(map[super.Type]int),
	}
}

func (d *DynamicValueBuilder) Write(val super.Value) {
	typ := val.Type()
	tag, ok := d.which[typ]
	if !ok {
		tag = len(d.values)
		d.values = append(d.values, NewValueBuilder(typ))
		d.which[typ] = tag
	}
	d.tags = append(d.tags, uint32(tag))
	d.values[tag].Write(val.Bytes())
}

func (d *DynamicValueBuilder) Build(sctx *super.Context) Any {
	var vecs []Any
	for _, b := range d.values {
		vecs = append(vecs, b.Build(sctx))
	}
	if len(vecs) == 1 {
		return vecs[0]
	}
	return NewDynamic(d.tags, vecs)
}

func NewValueBuilder(typ super.Type) ValueBuilder {
	switch typ := typ.(type) {
	case *super.TypeOfUint8,
		*super.TypeOfUint16,
		*super.TypeOfUint32,
		*super.TypeOfUint64:
		return &uintValueBuilder{typ: typ}
	case *super.TypeOfInt8,
		*super.TypeOfInt16,
		*super.TypeOfInt32,
		*super.TypeOfInt64,
		*super.TypeOfDuration,
		*super.TypeOfTime:
		return &intValueBuilder{typ: typ}
	case *super.TypeOfFloat16,
		*super.TypeOfFloat32,
		*super.TypeOfFloat64:
		return &floatValueBuilder{typ: typ}
	case *super.TypeOfBool:
		return newBoolValueBuilder()
	case *super.TypeOfBytes,
		*super.TypeOfString,
		*super.TypeOfAll:
		return newBytesStringValueBuilder(typ)
	case *super.TypeOfIP:
		return &ipValueBuilder{}
	case *super.TypeOfNet:
		return &netValueBuilder{}
	case *super.TypeOfType:
		return newTypeValueValueBuilder()
	case *super.TypeOfNone:
		return &noneValueBuilder{}
	case *super.TypeOfNull:
		return &nullValueBuilder{}
	case *super.TypeRecord:
		return newRecordValueBuilder(typ)
	case *super.TypeArray:
		return newArraySetValueBuilder(typ)
	case *super.TypeSet:
		return newArraySetValueBuilder(typ)
	case *super.TypeMap:
		return newMapValueBuilder(typ)
	case *super.TypeUnion:
		return newUnionValueBuilder(typ)
	case *super.TypeFusion:
		return newFusionValueBuilder(typ)
	case *super.TypeEnum:
		return &enumValueBuilder{typ, nil}
	case *super.TypeError:
		return &errorValueBuilder{typ: typ, ValueBuilder: NewValueBuilder(typ.Type)}
	case *super.TypeNamed:
		return &namedValueBuilder{typ: typ}
	}
	panic(typ)
}

type namedValueBuilder struct {
	builder ValueBuilder
	typ     *super.TypeNamed
}

func (n *namedValueBuilder) Write(bytes scode.Bytes) {
	if n.builder == nil {
		n.builder = NewValueBuilder(n.typ.Type)
	}
	n.builder.Write(bytes)
}

func (n *namedValueBuilder) Build(sctx *super.Context) Any {
	if n.builder == nil {
		return &Empty{n.typ}
	}
	return NewNamed(n.typ, n.builder.Build(sctx))
}

type recordValueBuilder struct {
	typ    *super.TypeRecord
	fields []ValueBuilder
	len    uint32
}

func newRecordValueBuilder(typ *super.TypeRecord) ValueBuilder {
	var fields []ValueBuilder
	for _, f := range typ.Fields {
		fields = append(fields, NewValueBuilder(f.Type))
	}
	return &recordValueBuilder{typ: typ, fields: fields}
}

func (r *recordValueBuilder) Write(bytes scode.Bytes) {
	r.len++
	it := bytes.Iter()
	for k := range r.fields {
		r.fields[k].Write(it.Next())
	}
}

func (r *recordValueBuilder) Build(sctx *super.Context) Any {
	var fields []Any
	for k := range r.fields {
		fields = append(fields, r.fields[k].Build(sctx))
	}
	return NewRecord(r.typ, fields, r.len)
}

type optionValueBuilder struct {
	union *super.TypeUnion
	val   *unionValueBuilder
	off   uint32
	runs  RLE
}

func (o *optionValueBuilder) Write(bytes scode.Bytes) {
	//XXX this isn't right and I don't think we need it.
	// CSUP can turn option unions into RLEs automatically
	off := o.off
	o.off++
	typ, _ := o.union.Untag(bytes)
	if typ == super.TypeNone {
		return
	}
	o.runs.Touch(off)
	o.val.Write(bytes)
}

func (o *optionValueBuilder) Build(sctx *super.Context) Any {
	// Strip the none type from the underlying union.
	vec := o.val.Build(sctx)
	n := vec.Len() //XXX
	return NewOptionFromRLE(sctx, vec, n, o.runs.End(n))
}

type errorValueBuilder struct {
	typ *super.TypeError
	ValueBuilder
}

func (e *errorValueBuilder) Build(sctx *super.Context) Any {
	return NewError(e.typ, e.ValueBuilder.Build(sctx))
}

type arraySetValueBuilder struct {
	typ     super.Type
	values  ValueBuilder
	offsets []uint32
}

func newArraySetValueBuilder(typ super.Type) ValueBuilder {
	return &arraySetValueBuilder{typ: typ, values: NewValueBuilder(super.InnerType(typ)), offsets: []uint32{0}}
}

func (a *arraySetValueBuilder) Write(bytes scode.Bytes) {
	off := a.offsets[len(a.offsets)-1]
	for it := bytes.Iter(); !it.Done(); {
		a.values.Write(it.Next())
		off++
	}
	a.offsets = append(a.offsets, off)
}

func (a *arraySetValueBuilder) Build(sctx *super.Context) Any {
	if typ, ok := a.typ.(*super.TypeArray); ok {
		return NewArray(typ, a.offsets, a.values.Build(sctx))
	}
	return NewSet(a.typ.(*super.TypeSet), a.offsets, a.values.Build(sctx))
}

type mapValueBuilder struct {
	typ          *super.TypeMap
	keys, values ValueBuilder
	offsets      []uint32
}

func newMapValueBuilder(typ *super.TypeMap) ValueBuilder {
	return &mapValueBuilder{
		typ:     typ,
		keys:    NewValueBuilder(typ.KeyType),
		values:  NewValueBuilder(typ.ValType),
		offsets: []uint32{0},
	}
}

func (m *mapValueBuilder) Write(bytes scode.Bytes) {
	off := m.offsets[len(m.offsets)-1]
	it := bytes.Iter()
	for !it.Done() {
		m.keys.Write(it.Next())
		m.values.Write(it.Next())
		off++
	}
	m.offsets = append(m.offsets, off)
}

func (m *mapValueBuilder) Build(sctx *super.Context) Any {
	return NewMap(m.typ, m.offsets, m.keys.Build(sctx), m.values.Build(sctx))
}

type unionValueBuilder struct {
	typ    *super.TypeUnion
	values []ValueBuilder
	tags   []uint32
}

func newUnionValueBuilder(typ *super.TypeUnion) ValueBuilder {

	var values []ValueBuilder
	for _, typ := range typ.Types {
		values = append(values, NewValueBuilder(typ))
	}
	builder := &unionValueBuilder{typ: typ, values: values}
	//if union, _ := super.OptionUnion(typ); union != nil {
	//	return &optionValueBuilder{union: union, val: builder}
	//}
	return builder
}

func (u *unionValueBuilder) Write(bytes scode.Bytes) {
	var typ super.Type
	typ, bytes = u.typ.Untag(bytes)
	tag := u.typ.TagOf(typ)
	u.values[tag].Write(bytes)
	u.tags = append(u.tags, uint32(tag))
}

// XXX optional should have two forms like typevalues...
// the union form with a type none in the union and the RLE form,
// with the underlying compact vector... we turn RLEs into tags
// on demand so that when we load record with lots of optional fields
// (as is common with fusion) then we only unroll the tags on demand
// and an optional field can round trip from vcache to runtime and back
// to CSUP without the tags ever being built.

func (u *unionValueBuilder) Build(sctx *super.Context) Any {
	// If there is a none type in this union, then we were wrapped
	// with an option type builder and we need to remove the none type
	// from the union.
	/* XXX we don't need this... just build the union
	if unionType, tag := super.OptionUnion(u.typ); u != nil {
		if len(unionType.Types) == 2 {
			var valTag int
			if tag == 0 {
				valTag = 1
			}
			return u.values[valTag].Build(sctx)
		}
		types := make([]super.Type, 0, len(unionType.Types)-1)
		vecs := make([]Any, 0, len(unionType.Types)-1)
		tags := make([][]uint32, 0, len(unionType.Types)-1)
		for k, t := range unionType.Types {
			if k != tag {
				vecs = append(vecs, u.values[k].Build(sctx))
				types = append(types, t)
				tags = append(tags, u.tags[k])
			}
		}
		return NewUnion(sctx.LookupTypeUnion(types), u.tags, vecs)
	}
	*/
	var vecs []Any
	for _, v := range u.values {
		vecs = append(vecs, v.Build(sctx))
	}
	//XXX maybe turn this into a vector.Option after the fact?
	return NewUnion(u.typ, u.tags, vecs)
}

type fusionValueBuilder struct {
	typ      *super.TypeFusion
	values   ValueBuilder
	subtypes []scode.Bytes
}

func newFusionValueBuilder(typ *super.TypeFusion) ValueBuilder {
	return &fusionValueBuilder{typ: typ, values: NewValueBuilder(typ.Type)}
}

func (f *fusionValueBuilder) Write(bytes scode.Bytes) {
	it := bytes.Iter()
	f.values.Write(it.Next())
	f.subtypes = append(f.subtypes, it.Next())
}

func (f *fusionValueBuilder) Build(sctx *super.Context) Any {
	types := make([]super.Type, 0, len(f.subtypes))
	for _, tv := range f.subtypes {
		t, err := sctx.LookupByValue(tv)
		if err != nil {
			panic(err)
		}
		types = append(types, t)
	}
	return NewFusion(sctx, f.typ, f.values.Build(sctx), types)
}

type enumValueBuilder struct {
	typ    *super.TypeEnum
	values []uint64
}

func (e *enumValueBuilder) Write(bytes scode.Bytes) {
	e.values = append(e.values, super.DecodeUint(bytes))
}

func (e *enumValueBuilder) Build(sctx *super.Context) Any {
	return NewEnum(e.typ, e.values)
}

type intValueBuilder struct {
	typ    super.Type
	values []int64
}

func (i *intValueBuilder) Write(bytes scode.Bytes) {
	i.values = append(i.values, super.DecodeInt(bytes))
}

func (i *intValueBuilder) Build(sctx *super.Context) Any {
	return NewInt(i.typ, i.values)
}

type uintValueBuilder struct {
	typ    super.Type
	values []uint64
}

func (u *uintValueBuilder) Write(bytes scode.Bytes) {
	u.values = append(u.values, super.DecodeUint(bytes))
}

func (u *uintValueBuilder) Build(sctx *super.Context) Any {
	return NewUint(u.typ, u.values)
}

type floatValueBuilder struct {
	typ    super.Type
	values []float64
}

func (f *floatValueBuilder) Write(bytes scode.Bytes) {
	f.values = append(f.values, super.DecodeFloat(bytes))
}

func (f *floatValueBuilder) Build(sctx *super.Context) Any {
	return NewFloat(f.typ, f.values)
}

type boolValueBuilder struct {
	values *roaring.Bitmap
	n      uint32
}

func newBoolValueBuilder() ValueBuilder {
	return &boolValueBuilder{values: roaring.New()}
}

func (b *boolValueBuilder) Write(bytes scode.Bytes) {
	if super.DecodeBool(bytes) {
		b.values.Add(b.n)
	}
	b.n++
}

func (b *boolValueBuilder) Build(sctx *super.Context) Any {
	bits := make([]uint64, (b.n+63)/64)
	b.values.WriteDenseTo(bits)
	return NewBool(bitvec.New(bits, b.n))
}

type typeValueValueBuilder struct {
	table BytesTable
}

func newTypeValueValueBuilder() *typeValueValueBuilder {
	return &typeValueValueBuilder{table: NewBytesTableEmpty(0)}
}

func (t *typeValueValueBuilder) Write(bytes scode.Bytes) {
	t.table.Append(bytes)
}

func (t *typeValueValueBuilder) Build(sctx *super.Context) Any {
	types := make([]super.Type, t.table.Len())
	for i := range t.table.Len() {
		var tv scode.Bytes
		types[i], tv = sctx.DecodeTypeValue(t.table.Bytes(i))
		if tv == nil {
			panic("bad type value")
		}
	}
	return NewTypeValue(sctx, types)
}

type bytesStringValueBuilder struct {
	typ   super.Type
	offs  []uint32
	bytes []byte
}

func newBytesStringValueBuilder(typ super.Type) ValueBuilder {
	return &bytesStringValueBuilder{typ: typ, bytes: []byte{}, offs: []uint32{0}}
}

func (b *bytesStringValueBuilder) Write(bytes scode.Bytes) {
	b.bytes = append(b.bytes, bytes...)
	b.offs = append(b.offs, uint32(len(b.bytes)))
}

func (b *bytesStringValueBuilder) Build(sctx *super.Context) Any {
	table := NewBytesTable(b.offs, b.bytes)
	switch b.typ.ID() {
	case super.IDString:
		return NewString(table)
	case super.IDBytes, super.IDAll:
		return NewBytes(table)
	default:
		panic(b.typ)
	}
}

type ipValueBuilder struct {
	values []netip.Addr
}

func (i *ipValueBuilder) Write(bytes scode.Bytes) {
	i.values = append(i.values, super.DecodeIP(bytes))
}

func (i *ipValueBuilder) Build(sctx *super.Context) Any {
	return NewIP(i.values)
}

type netValueBuilder struct {
	values []netip.Prefix
}

func (n *netValueBuilder) Write(bytes scode.Bytes) {
	n.values = append(n.values, super.DecodeNet(bytes))
}

func (n *netValueBuilder) Build(sctx *super.Context) Any {
	return NewNet(n.values)
}

type nullValueBuilder struct {
	n uint32
}

func (c *nullValueBuilder) Write(scode.Bytes) {
	c.n++
}

func (c *nullValueBuilder) Build(*super.Context) Any {
	return NewNull(c.n)
}

type noneValueBuilder struct {
	len uint32
}

func (n *noneValueBuilder) Write(scode.Bytes) {
	n.len++
}

func (n *noneValueBuilder) Build(*super.Context) Any {
	return NewNoneTmp(n.len)
}
