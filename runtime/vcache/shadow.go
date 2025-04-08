package vcache

import (
	"fmt"
	"sync"

	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
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
// minimal work unmarshaling the CSUP metadata as needed.  When processing a sequence
// of CSUP files with a single projection, the incremental capability is not important
// but when caching CSUP objects (e.g., in local from S3), multiple threads operating
// concurrently on a single object benefit from incremental unmarshaling.  This is especially
// important when processing thin projections over objects with lots of heteregenous types.
//
// Note that the shadow doesn't know about the query type context, thereby allowing the shadow
// to be shared across different queries.  Instead, the loader that builds a vector.Any
// is reponsible for computing the shared type from the shadow hierarchy.
//
// Shadows are created with unmarshal and only the portion of the shadow tree is
// created for the passed-in projection.  It is intended that a given shadow may be
// updated incrementally and concurrently but there some locking missing and this
// has not yet been tested so for now assume that unmarhsal here IS NOT REENTRANT.

type shadow interface {
	length() uint32
}

type dynamic struct {
	mu     sync.Mutex
	meta   *csup.Dynamic
	tags   []uint32 // need not be loaded for unordered dynamics
	values []shadow
}

func (d *dynamic) length() uint32 {
	return d.meta.Length
}

type record struct {
	mu   sync.Mutex
	meta *csup.Record
	count
	fields []shadow
	nulls  nulls
}

type array struct {
	mu   sync.Mutex
	meta *csup.Array
	count
	offs   []uint32
	values shadow
	nulls  nulls
}

type set struct {
	mu   sync.Mutex
	meta *csup.Set
	count
	offs   []uint32
	values shadow
	nulls  nulls
}

type union struct {
	mu   sync.Mutex
	meta *csup.Union
	count
	// XXX we should store TagMap here so it doesn't have to be recomputed
	tags   []uint32
	values []shadow
	nulls  nulls
}

type map_ struct {
	mu   sync.Mutex
	meta *csup.Map
	count
	offs   []uint32
	keys   shadow
	values shadow
	nulls  nulls
}

type primitive struct {
	mu   sync.Mutex
	meta *csup.Primitive
	count
	vec   vector.Any
	nulls nulls
}

type int_ struct {
	mu   sync.Mutex
	meta *csup.Int
	count
	vec   vector.Any
	nulls nulls
}

type uint_ struct {
	mu   sync.Mutex
	meta *csup.Uint
	count
	vec   vector.Any
	nulls nulls
}

type const_ struct {
	mu   sync.Mutex
	meta *csup.Const
	count
	vec   *vector.Const
	nulls nulls
}

type dict struct {
	mu   sync.Mutex
	meta *csup.Dict
	count
	nulls  nulls
	values shadow
	counts []uint32 // number of each entry indexed by dict offset
	index  []byte   // dict offset of each value in vector
}

type error_ struct {
	values shadow
	nulls  nulls
}

func (e *error_) length() uint32 {
	return e.values.length()
}

type named struct {
	meta   *csup.Named
	values shadow
}

func (n *named) length() uint32 {
	return n.values.length()
}

type count struct {
	vals  uint32
	nulls uint32
}

func (c count) length() uint32 {
	return c.nulls + c.vals
}

// unmarshal decodes the CSUP metadata structure to a (partial) shadow according
// to the provided projection.  No vector data is actually loaded.
// Nulls are read from storage and unwrapped
// so that all leaves of a given type have the same number of slots.  The vcache
// is then responsible for loading leaf vectors on demand as they are required
// by the runtime.
func unmarshal(cctx *csup.Context, id csup.ID, target shadow, projection field.Projection, n *csup.Nulls, nullsCnt uint32) shadow {
	switch meta := cctx.Lookup(id).(type) {
	case *csup.Dynamic:
		var d *dynamic
		if target == nil {
			d = &dynamic{
				meta:   meta,
				values: make([]shadow, len(meta.Values)),
			}
		} else {
			d = target.(*dynamic)
		}
		for k := range d.values {
			d.values[k] = unmarshal(cctx, meta.Values[k], d.values[k], projection, nil, 0)
		}
		return d
	case *csup.Nulls:
		if n != nil {
			panic("can't wrap nulls inside of nulls")
		}
		if target != nil {
			// We already processed this Nulls data so the target is whatever
			// it was transformed into.  Just recursively descend now.
			return unmarshal(cctx, meta.Values, target, projection, nil, 0)
		}
		return unmarshal(cctx, meta.Values, nil, projection, meta, nullsCnt+meta.Count)
	case *csup.Error:
		var e *error_
		if target == nil {
			e = &error_{nulls: nulls{meta: n}}
		} else {
			e = target.(*error_)
		}
		e.values = unmarshal(cctx, meta.Values, e.values, projection, nil, nullsCnt)
		return e
	case *csup.Named:
		var nm *named
		if target == nil {
			nm = &named{meta: meta}
		} else {
			nm = target.(*named)
		}
		nm.values = unmarshal(cctx, meta.Values, nm.values, projection, n, nullsCnt)
		return nm
	case *csup.Record:
		var r *record
		if target == nil {
			r = &record{
				meta:   meta,
				fields: make([]shadow, len(meta.Fields)),
				nulls:  nulls{meta: n},
				count:  count{meta.Length, nullsCnt},
			}
		} else {
			r = target.(*record)
		}
		if len(projection) == 0 {
			// Unmarshal all the fields of this record.  We're either loading all on demand (nil paths)
			// or loading this record because it's referenced at the end of a projected path.
			for k, f := range r.fields {
				r.fields[k] = unmarshal(cctx, meta.Fields[k].Values, f, nil, nil, r.count.nulls)
			}
			return r
		}
		switch elem := projection[0].(type) {
		case string:
			if k := indexOfField(elem, r.meta); k >= 0 {
				r.fields[k] = unmarshal(cctx, meta.Fields[k].Values, r.fields[k], nil, nil, r.count.nulls)
			}
		case field.Fork:
			// Multiple fields at this level are being projected.
			for _, path := range elem {
				// records require a field name path element (i.e., string)
				if name, ok := path[0].(string); ok {
					if k := indexOfField(name, r.meta); k >= 0 {
						r.fields[k] = unmarshal(cctx, meta.Fields[k].Values, r.fields[k], projection[1:], nil, r.count.nulls)
					}
				}
			}
		default:
			panic(fmt.Sprintf("bad path reference vcache record: %T", elem))
		}
		return r
	case *csup.Array:
		var a *array
		if target == nil {
			a = &array{
				meta:  meta,
				nulls: nulls{meta: n},
				count: count{meta.Length, nullsCnt},
			}
		} else {
			a = target.(*array)
		}
		a.values = unmarshal(cctx, meta.Values, a.values, nil, nil, 0)
		return a
	case *csup.Set:
		var s *set
		if target == nil {
			s = &set{
				meta:  meta,
				nulls: nulls{meta: n},
				count: count{meta.Length, nullsCnt},
			}
		} else {
			s = target.(*set)
		}
		s.values = unmarshal(cctx, meta.Values, s.values, nil, nil, 0)
		return s
	case *csup.Map:
		var m *map_
		if target == nil {
			m = &map_{
				meta:  meta,
				nulls: nulls{meta: n},
				count: count{meta.Length, nullsCnt},
			}
		} else {
			m = target.(*map_)
		}
		m.keys = unmarshal(cctx, meta.Keys, m.keys, nil, nil, 0)
		m.values = unmarshal(cctx, meta.Values, m.values, nil, nil, 0)
		return m
	case *csup.Union:
		var u *union
		if target == nil {
			u = &union{
				meta:   meta,
				values: make([]shadow, len(meta.Values)),
				nulls:  nulls{meta: n},
				count:  count{meta.Length, nullsCnt},
			}
		} else {
			u = target.(*union)
		}
		for k, id := range meta.Values {
			u.values[k] = unmarshal(cctx, id, u.values[k], projection, nil, 0)
		}
		return u
	case *csup.Int:
		var i *int_
		if target == nil {
			i = &int_{
				meta:  meta,
				nulls: nulls{meta: n},
				count: count{meta.Len(cctx), nullsCnt},
			}
		} else {
			i = target.(*int_)
		}
		return i
	case *csup.Uint:
		var u *uint_
		if target == nil {
			u = &uint_{
				meta:  meta,
				nulls: nulls{meta: n},
				count: count{meta.Len(cctx), nullsCnt},
			}
		} else {
			u = target.(*uint_)
		}
		return u
	case *csup.Primitive:
		var p *primitive
		if target == nil {
			p = &primitive{
				meta:  meta,
				nulls: nulls{meta: n},
				count: count{meta.Len(cctx), nullsCnt},
			}
		} else {
			p = target.(*primitive)
		}
		return p
	case *csup.Const:
		var c *const_
		if target == nil {
			c = &const_{
				meta:  meta,
				nulls: nulls{meta: n},
				count: count{meta.Len(cctx), nullsCnt},
			}
		} else {
			c = target.(*const_)
		}
		return c
	case *csup.Dict:
		var d *dict
		if target == nil {
			d = &dict{
				meta:  meta,
				nulls: nulls{meta: n},
				count: count{meta.Len(cctx), nullsCnt},
			}
		} else {
			d = target.(*dict)
		}
		d.values = unmarshal(cctx, meta.Values, d.values, projection, nil, 0)
		return d
	default:
		panic(fmt.Sprintf("vector cache: type %T not supported", meta))
	}
}
