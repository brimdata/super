package vcache

import (
	"fmt"
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/vector"
)

// The shadow type mirrors the vector.Any implementations here with locks and
// pointers to shared vector slices.  This lets us page in just the portions
// of vector data that is needed at any given time (which we cache inside the shadow).
// When we need a runtime vector, we build the immutable vector.Any components from
// mutable shadow pieces that are dynamically loaded and maintained here.
// The invariant is that runtime vectors are immutable while vcache.shadow
// vectors are mutated under locks here as needed.

type shadow interface {
	length() uint32
}

type dynamic struct {
	mu   sync.Mutex
	len  uint32
	tags []uint32 // need not be loaded for unordered dynamics
	loc  csup.Segment
	vals []shadow
}

func (d *dynamic) length() uint32 {
	return d.len
}

type record struct {
	count
	fields []field_
	nulls  nulls
}

type field_ struct {
	name string
	val  shadow
}

type array struct {
	mu sync.Mutex
	count
	loc   csup.Segment
	offs  []uint32
	vals  shadow
	nulls nulls
}

type set struct {
	mu sync.Mutex
	count
	loc   csup.Segment
	offs  []uint32
	vals  shadow
	nulls nulls
}

type union struct {
	mu sync.Mutex
	count
	// XXX we should store TagMap here so it doesn't have to be recomputed
	tags  []uint32
	loc   csup.Segment
	vals  []shadow
	nulls nulls
}

type map_ struct {
	mu sync.Mutex
	count
	offs  []uint32
	loc   csup.Segment
	keys  shadow
	vals  shadow
	nulls nulls
}

type primitive struct {
	mu sync.Mutex
	count
	csup  *csup.Primitive
	vec   vector.Any
	nulls nulls
}

type int_ struct {
	mu sync.Mutex
	count
	csup  *csup.Int
	vec   vector.Any
	nulls nulls
}

type uint_ struct {
	mu sync.Mutex
	count
	csup  *csup.Uint
	vec   vector.Any
	nulls nulls
}

type const_ struct {
	mu sync.Mutex
	count
	val super.Value //XXX map this value? XXX, maybe wrap a shadow vector?, which could
	// have a named in it
	vec   *vector.Const
	nulls nulls
}

type dict struct {
	mu sync.Mutex
	count
	csup   *csup.Dict
	nulls  nulls
	vals   shadow
	counts []uint32
	index  []byte
}

type error_ struct {
	vals  shadow
	nulls nulls
}

func (e *error_) length() uint32 {
	return e.vals.length()
}

type named struct {
	name string
	vals shadow
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

// newShadow converts the CSUP metadata structure to a complete vector.Any
// without loading any leaf columns.  Nulls are read from storage and unwrapped
// so that all leaves of a given type have the same number of slots.  The vcache
// is then responsible for loading leaf vectors on demand as they are required
// by the runtime.
func newShadow(m csup.Metadata, n *csup.Nulls, nullsCnt uint32) shadow {
	switch m := m.(type) {
	case *csup.Dynamic:
		vals := make([]shadow, 0, len(m.Values))
		for _, val := range m.Values {
			vals = append(vals, newShadow(val, nil, 0))
		}
		return &dynamic{
			vals: vals,
			len:  m.Len(),
			loc:  m.Tags,
		}
	case *csup.Nulls:
		if n != nil {
			panic("can't wrap nulls inside of nulls")
		}
		nullsCnt += m.Count
		return newShadow(m.Values, m, nullsCnt)
	case *csup.Error:
		return &error_{newShadow(m.Values, n, nullsCnt), nulls{meta: n}}
	case *csup.Named:
		return &named{m.Name, newShadow(m.Values, n, nullsCnt)}
	case *csup.Record:
		fields := make([]field_, 0, len(m.Fields))
		for _, f := range m.Fields {
			fields = append(fields, field_{f.Name, newShadow(f.Values, nil, nullsCnt)})
		}
		return &record{
			count:  count{m.Len(), nullsCnt},
			fields: fields,
			nulls:  nulls{meta: n},
		}
	case *csup.Array:
		return &array{
			count: count{m.Len(), nullsCnt},
			loc:   m.Lengths,
			vals:  newShadow(m.Values, nil, 0),
			nulls: nulls{meta: n},
		}
	case *csup.Set:
		return &set{
			count: count{m.Len(), nullsCnt},
			loc:   m.Lengths,
			vals:  newShadow(m.Values, nil, 0),
			nulls: nulls{meta: n},
		}
	case *csup.Map:
		return &map_{
			count: count{m.Len(), nullsCnt},
			loc:   m.Lengths,
			keys:  newShadow(m.Keys, nil, 0),
			vals:  newShadow(m.Values, nil, 0),
			nulls: nulls{meta: n},
		}
	case *csup.Union:
		vals := make([]shadow, 0, len(m.Values))
		for k := range m.Values {
			vals = append(vals, newShadow(m.Values[k], nil, 0))
		}
		return &union{
			count: count{m.Len(), nullsCnt},
			loc:   m.Tags,
			vals:  vals,
			nulls: nulls{meta: n},
		}
	case *csup.Int:
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
