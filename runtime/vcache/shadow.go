package vcache

import (
	"fmt"
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zcode"
	"github.com/brimdata/super/zson"
)

// The shadow type mirrors the vector.Any implementations here with locks and
// pointers to shared vector slices.  This lets us page in just the portions
// of vector data that is needed at any given time (which we cache inside the shadow).
// When we need a runtime vector, we build the immutable vector.Any components from
// mutable shadow pieces that are dynamically loaded and maintained here.
// The invariant is that runtime vectors are immutable while vcache.shadow
// vectors are mutated under locks here as needed.
//
// Shadows are created incrementally so that a sequence of projections will do the
// minimal work unmarshaling the csup metadata as needed.  When processing a sequence
// of csup files with a single projection, the incremental capability is not important
// but when caching csup objects (e.g., in local from S3), multiple threads operating
// concurrently on a single object benefit from incremental unmarshaling.  This is especially
// important when processing thin projections over objects with lots of heteregenous types.
//
// Note that the shadow doesn't know about the query type context, thereby allowing the shadow
// to be shared across different queries.  Instead, the loader that builds a vector.Any
// is reponsible for computing the shared type from the shadow hierarchy.

type shadow interface {
	length() uint32
}

type dynamic struct {
	mu   sync.Mutex
	len  uint32
	tags []uint32 // need not be loaded for unordered dynamics
	loc  csup.Segment
	vals []shadow
	meta []super.Value // metadata for each vals column
}

func (d *dynamic) length() uint32 {
	return d.len
}

type record struct {
	count
	fields []field_
	nulls  *nulls
}

type field_ struct {
	name string
	val  shadow
	meta super.Value
}

type array struct {
	mu sync.Mutex
	count
	loc   csup.Segment
	offs  []uint32
	vals  shadow
	meta  super.Value
	nulls *nulls
}

type set struct {
	mu sync.Mutex
	count
	loc   csup.Segment
	offs  []uint32
	vals  shadow
	meta  super.Value
	nulls *nulls
}

type union struct {
	mu sync.Mutex
	count
	// XXX we should store TagMap here so it doesn't have to be recomputed
	tags  []uint32
	loc   csup.Segment
	vals  []shadow
	meta  []super.Value
	nulls *nulls
}

type map_ struct {
	mu sync.Mutex
	count
	offs     []uint32
	loc      csup.Segment
	keys     shadow
	keyMeta  super.Value
	vals     shadow
	valsMeta super.Value
	nulls    *nulls
}

type primitive struct {
	mu sync.Mutex
	count
	csup  *csup.Primitive
	vec   vector.Any
	nulls *nulls
}

type int_ struct {
	mu sync.Mutex
	count
	min   int64
	max   int64
	loc   *csup.Segment
	vec   vector.Any
	nulls *nulls
}

type uint_ struct {
	mu sync.Mutex
	count
	csup  *csup.Uint
	vec   vector.Any
	nulls *nulls
}

type const_ struct {
	mu sync.Mutex
	count
	val super.Value //XXX map this value? XXX, maybe wrap a shadow vector?, which could
	// have a named in it
	vec   *vector.Const
	nulls *nulls
}

type dict struct {
	mu sync.Mutex
	count
	csup   *csup.Dict
	nulls  *nulls
	vals   shadow
	counts []uint32
	index  []byte
}

type error_ struct {
	vals  shadow
	meta  super.Value
	nulls *nulls
}

func (e *error_) length() uint32 {
	return e.vals.length()
}

type named struct {
	name string
	vals shadow
	meta super.Value
}

func (n *named) length() uint32 {
	return n.vals.length()
}

type count struct {
	vals  uint32
	nulls uint32
}

func (c count) length() uint32 {
	return c.nulls + c.vals
}

// XXX for now, no locking on shadow unmarshal.  just have a course lock
// cover the whole thing.  we can do finer-grained locking later.

// XXX update comment
// XXX return error instead of panic on corrupted csup data
// XXX newShadow -> fillShadow?  hydratePaths?
// newShadow converts the CSUP metadata structure to a complete vector.Any
// without loading any leaf columns.  Nulls are read from storage and unwrapped
// so that all leaves of a given type have the same number of slots.  The vcache
// is then responsible for loading leaf vectors on demand as they are required
// by the runtime.
func unmarshal(target shadow, meta super.Value, paths Path, n *nulls, nullsCnt uint32) shadow {
	metaTypeNamed, ok := meta.Type().(*super.TypeNamed)
	if !ok {
		panic("csup metadata value not a named type")
	}
	// Metadata entries are always records
	metaType, ok := metaTypeNamed.Type.(*super.TypeRecord)
	if !ok {
		//XXX return error
		panic(fmt.Sprint("csup metadata not a record: %s", zson.String(metaTypeNamed.Type)))
	}
	switch metaTypeNamed.Name {
	case "Dynamic":
		var d *dynamic
		if target != nil {
			d = target.(*dynamic)
		} else {
			d = unmarshalDynamic(metaType, meta.Bytes())
		}
		for k := range d.vals {
			d.vals[k] = unmarshal(d.vals[k], d.meta[k], paths, nil, 0)
		}
		return d
	case "Nulls":
		if n != nil {
			panic("can't wrap nulls inside of nulls")
		}
		//type Nulls struct {
		//	Runs   Segment
		//	Values Metadata
		//	Count  uint32 // Count of nulls
		//}
		var ns *nulls
		if target != nil {
			ns = target.(*nulls) // XXX add dummy length method?
		} else {
			ns = unmarshalNulls(metaType, meta.Bytes())
		}
		nullsCnt += ns.count
		return unmarshal(ns.vals, ns.meta, paths, ns, nullsCnt)
	case "Error":
		var e *error_
		if target != nil {
			e = target.(*error_)
		} else {
			e = unmarshalError(metaType, meta.Bytes(), n)
		}
		e.vals = unmarshal(e.vals, e.meta, paths, nil, nullsCnt)
		return e
	case "Named":
		var n *named
		if target != nil {
			n = target.(*named)
		} else {
			n = unmarshalNamed(metaType, meta.Bytes())
		}
		n.vals = unmarshal(n.vals, n.meta, paths, nil, nullsCnt)
		return n
	case "Record":
		var r *record
		if target != nil {
			r = target.(*record)
		} else {
			r = unmarshalRecord(metaType, meta.Bytes(), nullsCnt, n)
		}
		unmarshalProjection(r, paths)
		return r
	case "Array":
		var a *array
		if target != nil {
			a = target.(*array)
		} else {
			a = unmarshalArray(metaType, meta.Bytes(), nullsCnt, n)
		}
		//XXX note think about unnest of record elems inside of array by field
		a.vals = unmarshal(a.vals, a.meta, nil, nil, 0)
		return a
	case "Set":
		var s *set
		if target != nil {
			s = target.(*set)
		} else {
			s = unmarshalSet(metaType, meta.Bytes(), nullsCnt, n)
		}
		//XXX note think about unnest of record elems inside of array by field
		s.vals = unmarshal(s.vals, s.meta, nil, nil, 0)
		return s
	case "Map":
		var m *map_
		if target != nil {
			m = target.(*map_)
		} else {
			m = unmarshalMap(metaType, meta.Bytes(), nullsCnt, n)
		}
		//XXX note think about unnest of record elems inside of array by field
		m.keys = unmarshal(m.keys, m.keyMeta, nil, nil, 0)
		m.vals = unmarshal(m.vals, m.valsMeta, nil, nil, 0)
		return m
	case "Union":
		var u *union
		if target != nil {
			u = target.(*union)
		} else {
			u = unmarshalUnion(metaType, meta.Bytes(), nullsCnt, n)
		}
		for k := range u.vals {
			u.vals[k] = unmarshal(u.vals[k], u.meta[k], paths, nil, 0)
		}
		return u
	case *csup.Int:
		type Int struct {
			Typ      super.Type `zed:"Type"`
			Location Segment
			Min      int64
			Max      int64
			Count    uint32
		}
		return &int_{
			count: count{m.Len(), nullsCnt},
			csup:  m,
			nulls: nulls{meta: n},
		}
	case *csup.Uint:
		return &uint_{
			count: count{m.Len(), nullsCnt},
			csup:  m,
			nulls: nulls{meta: n},
		}
	case *csup.Primitive:
		return &primitive{
			count: count{m.Len(), nullsCnt},
			csup:  m,
			nulls: nulls{meta: n},
		}
	case *csup.Const:
		return &const_{
			count: count{m.Len(), nullsCnt},
			val:   m.Value,
			nulls: nulls{meta: n},
		}
	case *csup.Dict:
		return &dict{
			vals:  newShadow(m.Values, nil, 0),
			count: count{m.Len(), nullsCnt},
			csup:  m,
			nulls: nulls{meta: n},
		}
	default:
		panic(fmt.Sprintf("vector cache: type %T not supported", m))
	}
}

func unmarshalDynamic(typ *super.TypeRecord, bytes zcode.Bytes) *dynamic {
	var d dynamic
	//type Dynamic struct {
	//	Tags   Segment
	//	Values []Metadata
	//	Length uint32
	//}
	//XXX need concrete type of []Values
	arrayType, ok := typ.TypeOfField("Values")
	if !ok {
		panic("TBD")
	}
	elemType := arrayType.(*super.TypeArray).Type //XXX error
	it := zcode.Iter(bytes)
	unmarshalSegment(&d.loc, it.Next()) //XXX error
	array := it.Next().Iter()
	for !array.Done() {
		//XXX we need supervalues in the array so we can deunion etc.
		d.meta = append(d.meta, deunion(elemType, array.Next()))
	}
	d.len = uint32(super.DecodeUint(it.Next()))
	if !it.Done() {
		panic("TBD")
	}
	// Create vals array of proper length but filled with nils so we can hydrate on demand
	d.vals = make([]shadow, len(d.meta))
	return &d
}

func unmarshalRecord(typ *super.TypeRecord, bytes zcode.Bytes, nullsCnt uint32, nulls *nulls) *record {
	//type Record struct {
	//	Length uint32
	//	Fields []Field
	//}
	it := zcode.Iter(bytes)
	length := uint32(super.DecodeUint(it.Next()))
	arrayType, ok := typ.TypeOfField("Fields")
	if !ok {
		panic("TBD")
	}
	elemType := arrayType.(*super.TypeArray).Type //XXX error
	var fields []field_
	for _, f := range m.Fields {
		//type Field struct {
		//	Name   string
		//	Values Metadata
		//}
		//XXX decode FIeld...
		fit := it.Next().Iter()
		name := super.DecodeString(fit.Next())
		fields = append(fields, field_{
			name: name,
			meta: deunion(elemType, fit.Next()),
		})
	}
	return &record{
		count:  count{length, nullsCnt},
		fields: fields,
		nulls:  nulls,
	}
}

func unmarshalArray(typ *super.TypeRecord, bytes zcode.Bytes, nullsCnt uint32, nulls *nulls) *array {
	//type Array struct {
	//	Length  uint32
	//	Lengths Segment
	//	Values  Metadata
	//}
	it := zcode.Iter(bytes)
	var a array
	a.count = count{uint32(super.DecodeUint(it.Next())), nullsCnt}
	unmarshalSegment(&a.loc, it.Next())
	valType, ok := typ.TypeOfField("Values")
	if !ok {
		panic("TBD")
	}
	a.meta = super.NewValue(valType, it.Next())
	if !it.Done() {
		panic("TBD")
	}
	a.nulls = nulls
	return &a
}

func unmarshalSet(typ *super.TypeRecord, bytes zcode.Bytes, nullsCnt uint32, nulls *nulls) *set {
	//type Array struct {
	//	Length  uint32
	//	Lengths Segment
	//	Values  Metadata
	//}
	it := zcode.Iter(bytes)
	var s set
	s.count = count{uint32(super.DecodeUint(it.Next())), nullsCnt}
	unmarshalSegment(&s.loc, it.Next())
	valType, ok := typ.TypeOfField("Values")
	if !ok {
		panic("TBD")
	}
	s.meta = super.NewValue(valType, it.Next())
	if !it.Done() {
		panic("TBD")
	}
	s.nulls = nulls
	return &s
}

func unmarshalMap(typ *super.TypeRecord, bytes zcode.Bytes, nullsCnt uint32, nulls *nulls) *map_ {
	//type Map struct {
	//	Length  uint32
	//	Lengths Segment
	//	Keys    Metadata
	//	Values  Metadata
	//}
	it := zcode.Iter(bytes)
	var m map_
	m.count = count{uint32(super.DecodeUint(it.Next())), nullsCnt}
	unmarshalSegment(&m.loc, it.Next())
	keyType, ok := typ.TypeOfField("Keys")
	if !ok {
		panic("TBD")
	}
	m.keyMeta = super.NewValue(keyType, it.Next())
	valType, ok := typ.TypeOfField("Values")
	if !ok {
		panic("TBD")
	}
	m.valsMeta = super.NewValue(valType, it.Next())
	if !it.Done() {
		panic("TBD")
	}
	m.nulls = nulls
	return &m
}

func unmarshalUnion(typ *super.TypeRecord, bytes zcode.Bytes, nullsCnt uint32, nulls *nulls) *union {
	//type Union struct {
	//	Length uint32
	//	Tags   Segment
	//	Values []Metadata
	//}
	it := zcode.Iter(bytes)
	length := uint32(super.DecodeUint(it.Next()))
	arrayType, ok := typ.TypeOfField("Values")
	if !ok {
		panic("TBD")
	}
	elemType := arrayType.(*super.TypeArray).Type //XXX error
	array := it.Next().Iter()
	if !it.Done() {
		panic("TBD")
	}
	var meta []super.Value
	for !array.Done() {
		meta = append(meta, deunion(elemType, array.Next()))
	}
	return &union{
		count: count{length, nullsCnt},
		meta:  meta,
		nulls: nulls,
	}
}

func unmarshalNulls(typ *super.TypeRecord, bytes zcode.Bytes) *nulls {
	valType, ok := typ.TypeOfField("Value")
	if !ok {
		panic("TBD")
	}
	var n nulls
	//type Nulls struct {
	//	Runs   Segment
	//	Values Metadata
	//	Count  uint32 // Count of nulls
	//}
	//XXX need concrete type of []Values
	it := zcode.Iter(bytes)
	unmarshalSegment(&n.loc, it.Next()) //XXX error
	n.meta = super.NewValue(valType, it.Next())
	n.count = uint32(super.DecodeUint(it.Next()))
	if !it.Done() {
		panic("TBD")
	}
	return &n
}

func unmarshalError(typ *super.TypeRecord, bytes zcode.Bytes, nulls *nulls) *error_ {
	//type Error struct {
	//	Values Metadata
	//}
	valType, ok := typ.TypeOfField("Values")
	if !ok {
		panic("TBD")
	}
	it := zcode.Iter(bytes)
	val := super.NewValue(valType, it.Next())
	if !it.Done() {
		panic("TBD")
	}
	return &error_{
		meta:  val,
		nulls: nulls,
	}
}

func unmarshalNamed(typ *super.TypeRecord, bytes zcode.Bytes) *named {
	//type Named struct {
	//	Name   string
	//	Values Metadata
	//}
	valType, ok := typ.TypeOfField("Values")
	if !ok {
		panic("TBD")
	}
	it := zcode.Iter(bytes)
	name := super.DecodeString(it.Next())
	meta := super.NewValue(valType, it.Next())
	if !it.Done() {
		panic("TBD")
	}
	return &named{
		name: name,
		meta: meta,
	}
}

func unmarshalInt(typ *super.TypeRecord, bytes zcode.Bytes, nullsCnt uint32, nulls *nulls) *int_ {
	//type Int struct {
	//	Typ      super.Type `zed:"Type"`
	//	Location Segment
	//	Min      int64
	//	Max      int64
	//	Count    uint32
	//}
	var i int_
	it := zcode.Iter(bytes)
	typeval := super.DecodeTypeValue(it.Next()) //XXX look at unmarshal
	i.Typ = nil                                 //XXX lookup primitive typeval
	unmarshalSegment(i.loc, it.Next())
	i.min = super.Decodent(it.Next())
	i.max = super.Decodent(it.Next())
	meta := super.NewValue(valType, it.Next())
	if !it.Done() {
		panic("TBD")
	}
	return &int_{
		count: count{length, nullsCnt},
		min:   min,
		max:   max,
		nulls: nulls,
	}
}

// XXX error
func unmarshalSegment(dst *csup.Segment, bytes zcode.Bytes) {
	//type Segment struct {
	//	Offset            uint64 // Offset relative to start of file
	//	Length            uint64 // Length in file
	//	MemLength         uint64 // Length in memory
	//	CompressionFormat uint8  // Compression format in file
	//}
	it := bytes.Iter()
	dst.Offset = super.DecodeUint(it.Next())
	dst.Length = super.DecodeUint(it.Next())
	dst.MemLength = super.DecodeUint(it.Next())
	dst.CompressionFormat = uint8(super.DecodeUint(it.Next()))
	if !it.Done() {
		panic("TBD")
	}
}

func deunion(typ super.Type, b zcode.Bytes) super.Value {
	if union, ok := typ.(*super.TypeUnion); ok {
		typ, b = union.Untag(b)
	}
	return super.NewValue(typ, b)
}

func unmarshalProjection(r *record, paths Path) {
	if len(paths) == 0 {
		// Unmarshal all the fields of this record.  We're either loading all on demand (nil paths)
		// or loading this record because it's referenced at the end of a projected path.
		for k, f := range r.fields {
			r.fields[k].val = unmarshal(f.val, f.meta, nil, nil, 0)
		}
		return
	}
	switch elem := paths[0].(type) {
	case string:
		if k := indexOfField(elem, r.fields); k >= 0 {
			r.fields[k].val = unmarshal(r.fields[k].val, r.fields[k].meta, nil, nil, 0)
		}
	case Fork:
		// Multiple fields at this level are being projected.
		for _, path := range elem {
			// records require a field name path element (i.e., string)
			if name, ok := path[0].(string); ok {
				if k := indexOfField(name, r.fields); k >= 0 {
					r.fields[k].val = unmarshal(r.fields[k].val, r.fields[k].meta, paths[1:], nil, 0)
				}
			}
		}
	default:
		panic(fmt.Sprintf("bad path in vcache loadRecord: %T", elem))
	}
}
