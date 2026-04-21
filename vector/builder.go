package vector

import (
	"net/netip"
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/vector/bitvec"
)

type Builder interface {
	Write(Any)
	Build(*super.Context) Any
}

type DynamicBuilder struct {
	tags   []uint32
	values []Builder
	which  map[super.Type]uint32
}

func NewDynamicBuilder() *DynamicBuilder {
	return &DynamicBuilder{which: make(map[super.Type]uint32)}
}

func (d *DynamicBuilder) Write(vec Any) {
	if dynamic, ok := vec.(*Dynamic); ok {
		tagMap := make([]uint32, len(dynamic.Values))
		for i, vec := range dynamic.Values {
			if vec != nil {
				tagMap[i] = d.write(vec)
			}
		}
		for i := range vec.Len() {
			d.tags = append(d.tags, tagMap[dynamic.Tags[i]])
		}
	} else {
		tag := d.write(vec)
		for range vec.Len() {
			d.tags = append(d.tags, tag)
		}
	}
}

func (d *DynamicBuilder) write(vec Any) uint32 {
	typ := vec.Type()
	i, ok := d.which[typ]
	if !ok {
		i = uint32(len(d.values))
		d.which[typ] = i
		d.values = append(d.values, NewBuilder(typ))
	}
	d.values[i].Write(vec)
	return i
}

func (d *DynamicBuilder) Build(sctx *super.Context) Any {
	out := d.build(sctx)
	if len(out.Values) == 1 {
		return out.Values[0]
	}
	return out
}

func (d *DynamicBuilder) build(sctx *super.Context) *Dynamic {
	var vecs []Any
	for _, b := range d.values {
		vecs = append(vecs, b.Build(sctx))
	}
	return NewDynamic(d.tags, vecs)
}

func NewBuilder(typ super.Type) Builder {
	switch typ := typ.(type) {
	case *super.TypeOfUint8,
		*super.TypeOfUint16,
		*super.TypeOfUint32,
		*super.TypeOfUint64:
		return &genericBuilder[uint64]{
			typ:      typ,
			valuesOf: func(vec Any) []uint64 { return vec.(*Uint).Values },
			build: func(_ *super.Context, typ super.Type, vals []uint64) Any {
				return NewUint(typ, vals)
			},
		}
	case *super.TypeOfInt8,
		*super.TypeOfInt16,
		*super.TypeOfInt32,
		*super.TypeOfInt64,
		*super.TypeOfDuration,
		*super.TypeOfTime:
		return &genericBuilder[int64]{
			typ:      typ,
			valuesOf: func(vec Any) []int64 { return vec.(*Int).Values },
			build: func(_ *super.Context, typ super.Type, vals []int64) Any {
				return NewInt(typ, vals)
			},
		}
	case *super.TypeOfFloat16,
		*super.TypeOfFloat32,
		*super.TypeOfFloat64:
		return &genericBuilder[float64]{
			typ:      typ,
			valuesOf: func(vec Any) []float64 { return vec.(*Float).Values },
			build: func(_ *super.Context, typ super.Type, vals []float64) Any {
				return NewFloat(typ, vals)
			},
		}
	case *super.TypeOfBool:
		return &boolBuilder{}
	case *super.TypeOfString,
		*super.TypeOfBytes:
		return newStringBytesBuilder(typ)
	case *super.TypeOfIP:
		return &genericBuilder[netip.Addr]{
			typ:      typ,
			valuesOf: func(vec Any) []netip.Addr { return vec.(*IP).Values },
			build: func(_ *super.Context, _ super.Type, vals []netip.Addr) Any {
				return NewIP(vals)
			},
		}
	case *super.TypeOfNet:
		return &genericBuilder[netip.Prefix]{
			typ:      typ,
			valuesOf: func(vec Any) []netip.Prefix { return vec.(*Net).Values },
			build: func(_ *super.Context, _ super.Type, vals []netip.Prefix) Any {
				return NewNet(vals)
			},
		}
	case *super.TypeOfType:
		return &genericBuilder[super.Type]{
			valuesOf: func(vec Any) []super.Type { return vec.(*TypeValue).types },
			build: func(sctx *super.Context, _ super.Type, vals []super.Type) Any {
				return NewTypeValue(sctx, vals)
			},
		}
	case *super.TypeOfNull:
		return &nullBuilder{}
	case *super.TypeOfNone:
		return &noneBuilder{}
	case *super.TypeRecord:
		return newRecordBuilder(typ)
	case *super.TypeArray, *super.TypeSet:
		return newArraySetBuilder(typ)
	case *super.TypeMap:
		return newMapBuilder(typ)
	case *super.TypeUnion:
		return &unionBuilder{typ: typ, builder: NewDynamicBuilder()}
	case *super.TypeEnum:
		return newEnumBuilder(typ)
	case *super.TypeError:
		return &errorBuilder{vals: NewBuilder(typ.Type)}
	case *super.TypeNamed:
		return &namedBuilder{name: typ.Name, vals: NewBuilder(typ.Type)}
	case *super.TypeFusion:
		return &fusionBuilder{vals: NewBuilder(typ.Type)}
	default:
		panic(typ)
	}
}

type genericBuilder[E any] struct {
	typ      super.Type
	vals     []E
	valuesOf func(Any) []E
	build    func(*super.Context, super.Type, []E) Any
}

func (b *genericBuilder[E]) Write(vec Any) {
	if vec.Len() == 0 {
		return
	}
	switch vec := vec.(type) {
	case *View:
		vals := b.valuesOf(vec.Any)
		for _, slot := range vec.Index {
			b.vals = append(b.vals, vals[slot])
		}
	case *Const:
		vals := b.valuesOf(vec.Any)
		for range vec.len {
			b.vals = append(b.vals, vals[0])
		}
	case *Dict:
		vals := b.valuesOf(vec.Any)
		for _, slot := range vec.Index {
			b.vals = append(b.vals, vals[slot])
		}
	default:
		b.vals = append(b.vals, b.valuesOf(vec)...)
	}
}

func (b *genericBuilder[E]) Build(sctx *super.Context) Any {
	return b.build(sctx, b.typ, b.vals)
}

type boolBuilder struct {
	bits bitvec.Bits
}

func (b *boolBuilder) Write(vec Any) {
	switch vec := vec.(type) {
	case *Const:
		v := vec.Any.(*Bool).IsSet(0)
		for range vec.len {
			b.bits.Append(v)
		}
	case *Bool:
		// There's a faster way to do this with bit shift but just go slow for
		// now.
		for i := range vec.Len() {
			b.bits.Append(vec.IsSet(i))
		}
	}
}

func (b *boolBuilder) Build(*super.Context) Any {
	return NewBool(b.bits)
}

type stringBytesBuilder struct {
	typ   super.Type
	table BytesTable
}

func newStringBytesBuilder(typ super.Type) Builder {
	return &stringBytesBuilder{typ: typ, table: NewBytesTableEmpty(0)}
}

func (s *stringBytesBuilder) Write(vec Any) {
	switch vec := vec.(type) {
	case *View:
		table := bytesTableOf(vec.Any)
		for _, slot := range vec.Index {
			s.table.Append(table.Bytes(slot))
		}
	case *Const:
		b := bytesTableOf(vec.Any).Bytes(0)
		for range vec.len {
			s.table.Append(b)
		}
	case *Dict:
		table := bytesTableOf(vec.Any)
		for _, slot := range vec.Index {
			s.table.Append(table.Bytes(uint32(slot)))
		}
	case *String, *Bytes:
		table := bytesTableOf(vec)
		for i := range vec.Len() {
			s.table.Append(table.Bytes(i))
		}
	default:
		panic(vec)
	}
}

func bytesTableOf(vec Any) BytesTable {
	switch vec := vec.(type) {
	case *String:
		return vec.table
	case *Bytes:
		return vec.table
	case *Empty:
		return BytesTable{}
	default:
		panic(vec)
	}
}

func (s *stringBytesBuilder) Build(*super.Context) Any {
	switch s.typ.ID() {
	case super.IDString:
		return NewString(s.table)
	case super.IDBytes:
		return NewBytes(s.table)
	default:
		panic(s.typ)
	}
}

type noneBuilder struct {
	len uint32
}

func (n *noneBuilder) Write(vec Any) {
	n.len += vec.(*NoneTmp).len
}

func (n *noneBuilder) Build(*super.Context) Any {
	return NewNoneTmp(n.len)
}

type nullBuilder struct {
	len uint32
}

func (n *nullBuilder) Write(vec Any) {
	n.len += vec.(*Null).len
}

func (n *nullBuilder) Build(*super.Context) Any {
	return NewNull(n.len)
}

type arraySetBuilder struct {
	typ     super.Type
	inner   Builder
	offsets []uint32
	len     uint32
}

func newArraySetBuilder(typ super.Type) *arraySetBuilder {
	return &arraySetBuilder{
		typ:     typ,
		inner:   NewBuilder(super.InnerType(typ)),
		offsets: []uint32{0},
	}
}

func (a *arraySetBuilder) Write(vec Any) {
	n := vec.Len()
	var index []uint32
	if view, ok := vec.(*View); ok {
		vec = view.Any
		index = view.Index
	}
	var offsets []uint32
	switch vec := vec.(type) {
	case *Array:
		a.inner.Write(vec.Values)
		offsets = vec.Offsets
	case *Set:
		a.inner.Write(vec.Values)
		offsets = vec.Offsets
	default:
		panic(vec)
	}
	for i := range n {
		idx := i
		if index != nil {
			idx = index[i]
		}
		a.len += offsets[idx+1] - offsets[idx]
		a.offsets = append(a.offsets, a.len)
	}
}

func (a *arraySetBuilder) Build(sctx *super.Context) Any {
	vec := a.inner.Build(sctx)
	switch a.typ.(type) {
	case *super.TypeArray:
		typ := sctx.LookupTypeArray(vec.Type())
		return NewArray(typ, a.offsets, vec)
	case *super.TypeSet:
		typ := sctx.LookupTypeSet(vec.Type())
		return NewSet(typ, a.offsets, vec)
	default:
		panic(a.typ)
	}
}

type enumBuilder struct {
	typ  *super.TypeEnum
	uint Builder
}

func newEnumBuilder(typ *super.TypeEnum) Builder {
	return &enumBuilder{
		typ:  typ,
		uint: NewBuilder(super.TypeUint64),
	}
}

func (a *enumBuilder) Write(vec Any) {
	var index []uint32
	if view, ok := vec.(*View); ok {
		index = view.Index
		vec = view.Any
	}
	var vals Any = vec.(*Enum).Uint
	if index != nil {
		vals = Pick(vals, index)
	}
	a.uint.Write(vals)
}

func (a *enumBuilder) Build(sctx *super.Context) Any {
	return NewEnum(a.typ, a.uint.Build(sctx).(*Uint).Values)
}

type errorBuilder struct {
	vals Builder
}

func (e *errorBuilder) Write(vec Any) {
	e.vals.Write(vec.(*Error).Vals)
}

func (e *errorBuilder) Build(sctx *super.Context) Any {
	vals := e.vals.Build(sctx)
	return NewError(sctx.LookupTypeError(vals.Type()), vals)
}

type namedBuilder struct {
	name string
	vals Builder
}

func (n *namedBuilder) Write(vec Any) {
	n.vals.Write(vec.(*Named).Any)
}

func (n *namedBuilder) Build(sctx *super.Context) Any {
	vals := n.vals.Build(sctx)
	typ, err := sctx.LookupTypeNamed(n.name, vals.Type())
	if err != nil {
		panic(err)
	}
	return NewNamed(typ, vals)
}

type fusionBuilder struct {
	typ      *super.TypeFusion
	vals     Builder
	subtypes []super.Type
}

func (f *fusionBuilder) Write(vec Any) {
	fusion := vec.(*Fusion)
	f.typ = fusion.Typ
	f.vals.Write(fusion.Values)
	f.subtypes = append(f.subtypes, fusion.Subtypes.Types()...)
}

func (f *fusionBuilder) Build(sctx *super.Context) Any {
	vals := f.vals.Build(sctx)
	return NewFusion(sctx, f.typ, vals, f.subtypes)
}

type mapBuilder struct {
	keys    Builder
	vals    Builder
	offsets []uint32
	len     uint32
}

func newMapBuilder(typ *super.TypeMap) Builder {
	return &mapBuilder{
		keys:    NewBuilder(typ.KeyType),
		vals:    NewBuilder(typ.ValType),
		offsets: []uint32{0},
	}
}

func (m *mapBuilder) Write(vec Any) {
	n := vec.Len()
	var index []uint32
	if view, ok := vec.(*View); ok {
		index = view.Index
		vec = view.Any
	}
	vmap := vec.(*Map)
	m.keys.Write(vmap.Keys)
	m.vals.Write(vmap.Values)
	for i := range n {
		idx := i
		if index != nil {
			idx = index[i]
		}
		m.len += vmap.Offsets[idx+1] - vmap.Offsets[idx]
		m.offsets = append(m.offsets, m.len)
	}
}

func (m *mapBuilder) Build(sctx *super.Context) Any {
	keys := m.keys.Build(sctx)
	vals := m.vals.Build(sctx)
	typ := sctx.LookupTypeMap(keys.Type(), vals.Type())
	return NewMap(typ, m.offsets, keys, vals)
}

type recordBuilder struct {
	typ    *super.TypeRecord
	fields []Builder
	len    uint32
}

func newRecordBuilder(typ *super.TypeRecord) Builder {
	var fields []Builder
	for _, f := range typ.Fields {
		b := NewBuilder(f.Type)
		if f.Opt {
			b = &optionalBuilder{value: b}
		}
		fields = append(fields, b)
	}
	return &recordBuilder{
		typ:    typ,
		fields: fields,
	}
}

func (r *recordBuilder) Write(in Any) {
	vec := in
	var index []uint32
	if view, ok := vec.(*View); ok {
		vec = view.Any
		index = view.Index
	}
	rec := vec.(*Record)
	for i, vec := range rec.Fields {
		if index != nil {
			// XXX Optionals will return a dynamic.
			vec = Pick(vec, index)
		}
		r.fields[i].Write(vec)
	}
	r.len += in.Len()
}

func (r *recordBuilder) Build(sctx *super.Context) Any {
	var fields []Any
	for _, b := range r.fields {
		fields = append(fields, b.Build(sctx))
	}
	return NewRecord(r.typ, fields, r.len)
}

type optionalBuilder struct {
	rle   RLE
	value Builder
	len   uint32
}

func (o *optionalBuilder) Write(vec Any) {
	switch vec := vec.(type) {
	case *Optional:
		for i, tag := range vec.Tags {
			if tag == 0 {
				o.rle.Touch(o.len + uint32(i))
			}
		}
		o.value.Write(vec.Dynamic.Values[0])
	case *None: // Does nothing.
	default:
		for i := range vec.Len() {
			o.rle.Touch(o.len + i)
		}
		o.value.Write(vec)
	}
	o.len += vec.Len()
}

func (o *optionalBuilder) Build(sctx *super.Context) Any {
	rle := o.rle.End(o.len)
	return NewFieldFromRLE(sctx, o.value.Build(sctx), o.len, rle)
}

type unionBuilder struct {
	typ *super.TypeUnion

	builder *DynamicBuilder
}

func (u *unionBuilder) Write(vec Any) {
	// Assert all incoming types in union.
	vec = Deunion(vec)
	check := []Any{vec}
	if d, ok := vec.(*Dynamic); ok {
		check = d.Values
	}
	bad := slices.ContainsFunc(check, func(vec Any) bool {
		return u.typ.TagOf(vec.Type()) == -1
	})
	if bad {
		panic("incoming vector contains values not in union")
	}
	u.builder.Write(vec)
}

func (u *unionBuilder) Build(sctx *super.Context) Any {
	return NewUnionFromDynamic(sctx, u.builder.build(sctx))
}
