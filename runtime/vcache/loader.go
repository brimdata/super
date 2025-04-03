package vcache

import (
	"fmt"
	"io"
	"net/netip"
	"slices"
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zcode"
	"github.com/ronanh/intcomp"
	"golang.org/x/sync/errgroup"
)

// loader handles loading vector data on demand for only the fields needed
// as specified in the projection.  Each load is executed with a multiphase
// process: first, we build a mirror of the CSUP metadata where each node has a
// lock and places to store the bulk data so that it may be reused across
// projections.  This is called the shadow object.  Then, we fill in the shadow
// with data vectors dynamically and create runtime vectors as follows:
//
//	(1) create a mirror data structure (shadow)
//	(2) concurrently load all the nulls and tags, lens, etc. that will be needed (fetchNulls)
//	(3) compute top-down flattening of nulls concurrently (flatten)
//	(4) load all data that is projected using the nulls to flatten any unloaded data (fetchVals)
//	(5) form a projection from the fully loaded data nodes (project)
//
// The zctx passed into the loader is dynamic and comes from each query context that
// uses the vcache.  No zctx types are stored in the shadow (except for primitive types
// in shadowed vector.Any primitives that are shared).  We otherwise allocate all
// vector.Any super.Types using the passed-in zctx.
type loader struct {
	zctx *super.Context
	r    io.ReaderAt
}

// Load all vector data into the in-memory shadow that is needed and not yet loaded
// and return a new vector.Any using the data vectors in cache.  This may be called
// concurrently on the same shadow and fine-grained locking insures that any given
// data vector is loaded just once and such loads may be executed concurrently (even
// when only one thread is calling load).  If paths is nil, then the entire value
// is loaded.
func (l *loader) load(paths Path, s shadow) (vector.Any, error) {
	var group errgroup.Group
	l.fetchNulls(&group, paths, s)
	if err := group.Wait(); err != nil {
		return nil, err
	}
	flattenNulls(paths, s, nil)
	l.loadVector(&group, paths, s)
	if err := group.Wait(); err != nil {
		return nil, err
	}
	return project(l.zctx, paths, s), nil
}

func (l *loader) loadVector(g *errgroup.Group, paths Path, s shadow) {
	switch s := s.(type) {
	case *dynamic:
		//XXX we need an ordered option to load tags only when needed
		l.loadUint32(g, &s.mu, &s.tags, s.loc)
		for _, m := range s.vals {
			l.loadVector(g, paths, m)
		}
	case *record:
		l.loadRecord(g, paths, s)
	case *array:
		l.loadOffsets(g, &s.mu, &s.offs, s.loc, s.length(), s.nulls.flat)
		l.loadVector(g, paths, s.vals)
	case *set:
		l.loadOffsets(g, &s.mu, &s.offs, s.loc, s.length(), s.nulls.flat)
		l.loadVector(g, paths, s.vals)
	case *map_:
		l.loadOffsets(g, &s.mu, &s.offs, s.loc, s.length(), s.nulls.flat)
		l.loadVector(g, paths, s.keys)
		l.loadVector(g, paths, s.vals)
	case *union:
		l.loadUnion(g, paths, s)
	case *int_:
		l.loadInt(g, s)
	case *uint_:
		l.loadUint(g, s)
	case *primitive:
		l.loadPrimitive(g, paths, s)
	case *const_:
		s.mu.Lock()
		vec := s.vec
		if vec == nil {
			vec = vector.NewConst(s.val, s.length(), s.nulls.flat)
			s.vec = vec
		}
		s.mu.Unlock()
	case *dict:
		l.loadDict(g, paths, s)
	case *error_:
		l.loadVector(g, paths, s.vals)
	case *named:
		l.loadVector(g, paths, s.vals)
	default:
		panic(fmt.Sprintf("vector cache: shadow type %T not supported", s))
	}
}

func (l *loader) loadRecord(g *errgroup.Group, paths Path, s *record) {
	if len(paths) == 0 {
		// Load the whole record.  We're either loading all on demand (nil paths)
		// or loading this record because it's referenced at the end of a projected path.
		for _, f := range s.fields {
			l.loadVector(g, nil, f.val)
		}
		return
	}
	switch elem := paths[0].(type) {
	case string:
		if k := indexOfField(elem, s.fields); k >= 0 {
			l.loadVector(g, paths[1:], s.fields[k].val)
		}
	case Fork:
		// Multiple fields at this level are being projected.
		for _, path := range elem {
			// records require a field name path element (i.e., string)
			if name, ok := path[0].(string); ok {
				if k := indexOfField(name, s.fields); k >= 0 {
					l.loadVector(g, paths[1:], s.fields[k].val)
				}
			}
		}
	default:
		panic(fmt.Sprintf("bad path in vcache loadRecord: %T", elem))
	}
}

func (l *loader) loadUnion(g *errgroup.Group, paths Path, s *union) {
	l.loadUint32(g, &s.mu, &s.tags, s.loc)
	for _, val := range s.vals {
		l.loadVector(g, paths, val)
	}
}

func (l *loader) loadInt(g *errgroup.Group, s *int_) {
	s.mu.Lock()
	if s.vec != nil {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	g.Go(func() error {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.vec != nil {
			return nil
		}
		bytes := make([]byte, s.csup.Location.MemLength)
		if err := s.csup.Location.Read(l.r, bytes); err != nil {
			return err
		}
		vals := intcomp.UncompressInt64(byteconv.ReinterpretSlice[uint64](bytes), nil)
		vals = extendForNulls(vals, s.nulls.flat, s.count)
		s.vec = vector.NewInt(s.csup.Type(l.zctx), vals, s.nulls.flat)
		return nil
	})
}

func (l *loader) loadUint(g *errgroup.Group, s *uint_) {
	s.mu.Lock()
	if s.vec != nil {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	g.Go(func() error {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.vec != nil {
			return nil
		}
		bytes := make([]byte, s.csup.Location.MemLength)
		if err := s.csup.Location.Read(l.r, bytes); err != nil {
			return err
		}
		vals := intcomp.UncompressUint64(byteconv.ReinterpretSlice[uint64](bytes), nil)
		vals = extendForNulls(vals, s.nulls.flat, s.count)
		s.vec = vector.NewUint(s.csup.Type(l.zctx), vals, s.nulls.flat)
		return nil
	})
}

func (l *loader) loadPrimitive(g *errgroup.Group, paths Path, s *primitive) {
	s.mu.Lock()
	if s.vec != nil {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	g.Go(func() error {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.vec != nil {
			return nil
		}
		typ := s.csup.Type(l.zctx)
		vec, err := l.loadVals(typ, s, s.nulls.flat)
		if err != nil {
			return err
		}
		s.vec = vec
		return nil
	})
}

func (l *loader) loadVals(typ super.Type, s *primitive, nulls *vector.Bool) (vector.Any, error) {
	if s.csup.Count == 0 {
		return empty(typ, s.length(), nulls), nil
	}
	bytes := make([]byte, s.csup.Location.MemLength)
	if err := s.csup.Location.Read(l.r, bytes); err != nil {
		return nil, err
	}
	length := s.length()
	if nulls != nil && nulls.Len() != length {
		panic(fmt.Sprintf("BAD NULLS LEN nulls %d %d (cnt.vals %d cnt.null %d) %s", nulls.Len(), length, s.count.vals, s.count.nulls, sup.String(typ)))
	}
	it := zcode.Iter(bytes)
	switch typ := typ.(type) {
	case *super.TypeOfUint8, *super.TypeOfUint16, *super.TypeOfUint32, *super.TypeOfUint64:
		values := make([]uint64, length)
		for slot := uint32(0); slot < length; slot++ {
			if nulls == nil || !nulls.Value(slot) {
				values[slot] = super.DecodeUint(it.Next())
			}
		}
		return vector.NewUint(typ, values, nulls), nil
	case *super.TypeOfInt8, *super.TypeOfInt16, *super.TypeOfInt32, *super.TypeOfInt64, *super.TypeOfDuration, *super.TypeOfTime:
		values := make([]int64, length)
		for slot := uint32(0); slot < length; slot++ {

			if nulls == nil || !nulls.Value(slot) {
				values[slot] = super.DecodeInt(it.Next())
			}
		}
		return vector.NewInt(typ, values, nulls), nil
	case *super.TypeOfFloat16, *super.TypeOfFloat32, *super.TypeOfFloat64:
		values := make([]float64, length)
		for slot := uint32(0); slot < length; slot++ {
			if nulls == nil || !nulls.Value(slot) {
				values[slot] = super.DecodeFloat(it.Next())
			}
		}
		return vector.NewFloat(typ, values, nulls), nil
	case *super.TypeOfBool:
		b := vector.NewBoolEmpty(length, nulls)
		for slot := uint32(0); slot < length; slot++ {
			if nulls == nil || !nulls.Value(slot) {
				if super.DecodeBool(it.Next()) {
					b.Set(slot)
				}
			}
		}
		return b, nil
	case *super.TypeOfBytes:
		var bytes []byte
		offs := make([]uint32, length+1)
		var off uint32
		for slot := uint32(0); slot < length; slot++ {
			offs[slot] = off
			if nulls == nil || !nulls.Value(slot) {
				b := super.DecodeBytes(it.Next())
				bytes = append(bytes, b...)
				off += uint32(len(b))
			}
		}
		offs[length] = off
		return vector.NewBytes(offs, bytes, nulls), nil
	case *super.TypeOfString:
		var bytes []byte
		offs := make([]uint32, length+1)
		var off uint32
		for slot := uint32(0); slot < length; slot++ {
			offs[slot] = off
			if nulls == nil || !nulls.Value(slot) {
				s := super.DecodeString(it.Next())
				bytes = append(bytes, []byte(s)...)
				off += uint32(len(s))
			}
		}
		offs[length] = off
		return vector.NewString(offs, bytes, nulls), nil
	case *super.TypeOfIP:
		values := make([]netip.Addr, length)
		for slot := uint32(0); slot < length; slot++ {
			if nulls == nil || !nulls.Value(slot) {
				values[slot] = super.DecodeIP(it.Next())
			}
		}
		return vector.NewIP(values, nulls), nil
	case *super.TypeOfNet:
		values := make([]netip.Prefix, length)
		for slot := uint32(0); slot < length; slot++ {
			if nulls == nil || !nulls.Value(slot) {
				values[slot] = super.DecodeNet(it.Next())
			}
		}
		return vector.NewNet(values, nulls), nil
	case *super.TypeOfType:
		var bytes []byte
		offs := make([]uint32, length+1)
		var off uint32
		for slot := uint32(0); slot < length; slot++ {
			offs[slot] = off
			if nulls == nil || !nulls.Value(slot) {
				tv := it.Next()
				bytes = append(bytes, tv...)
				off += uint32(len(tv))
			}
		}
		offs[length] = off
		return vector.NewTypeValue(offs, bytes, nulls), nil
	case *super.TypeEnum:
		values := make([]uint64, length)
		for slot := range length {
			if !nulls.Value(slot) {
				values[slot] = super.DecodeUint(it.Next())
			}
		}
		return vector.NewEnum(typ, values, nulls), nil
	case *super.TypeOfNull:
		return vector.NewConst(super.Null, s.length(), nil), nil
	}
	return nil, fmt.Errorf("internal error: vcache.loadPrimitive got unknown type %#v", typ)
}

func (l *loader) loadDict(g *errgroup.Group, paths Path, s *dict) {
	if s.csup.Length == 0 {
		panic("empty dict") // empty dictionaries should not happen!
	}
	l.loadVector(g, paths, s.vals)
	l.loadUint32(g, &s.mu, &s.counts, s.csup.Counts)
	g.Go(func() error {
		s.mu.Lock()
		defer s.mu.Unlock()
		loc := s.csup.Index
		s.index = make([]byte, loc.MemLength)
		if err := loc.Read(l.r, s.index); err != nil {
			return err
		}
		s.index = extendForNulls(s.index, s.nulls.flat, s.count)
		return nil
	})

}

func extendForNulls[T any](in []T, nulls *vector.Bool, count count) []T {
	if count.nulls == 0 {
		return in
	}
	out := make([]T, count.length())
	var off int
	for i := range count.length() {
		if !nulls.Value(i) {
			out[i] = in[off]
			off++
		}
	}
	return out
}

// XXX need nullscnt to pass as length (ugh, need empty buffer nullscnt long because of flattened assumption)
func empty(typ super.Type, length uint32, nulls *vector.Bool) vector.Any {
	switch typ := typ.(type) {
	case *super.TypeOfUint8, *super.TypeOfUint16, *super.TypeOfUint32, *super.TypeOfUint64:
		return vector.NewUint(typ, make([]uint64, length), nulls)
	case *super.TypeOfInt8, *super.TypeOfInt16, *super.TypeOfInt32, *super.TypeOfInt64, *super.TypeOfDuration, *super.TypeOfTime:
		return vector.NewInt(typ, make([]int64, length), nulls)
	case *super.TypeOfFloat16, *super.TypeOfFloat32, *super.TypeOfFloat64:
		return vector.NewFloat(typ, make([]float64, length), nulls)
	case *super.TypeOfBool:
		return vector.NewBool(make([]uint64, (length+63)/64), length, nulls)
	case *super.TypeOfBytes:
		return vector.NewBytes(make([]uint32, length+1), nil, nulls)
	case *super.TypeOfString:
		return vector.NewString(make([]uint32, length+1), nil, nulls)
	case *super.TypeOfIP:
		return vector.NewIP(make([]netip.Addr, length), nulls)
	case *super.TypeOfNet:
		return vector.NewNet(make([]netip.Prefix, length), nulls)
	case *super.TypeOfType:
		return vector.NewTypeValue(make([]uint32, length+1), nil, nulls)
	case *super.TypeOfNull:
		return vector.NewConst(super.Null, length, nil)
	default:
		panic(fmt.Sprintf("vcache.empty: unknown type encountered: %T", typ))
	}
}

func (l *loader) loadUint32(g *errgroup.Group, mu *sync.Mutex, slice *[]uint32, loc csup.Segment) {
	mu.Lock()
	if *slice != nil {
		mu.Unlock()
		return
	}
	mu.Unlock()
	g.Go(func() error {
		mu.Lock()
		defer mu.Unlock()
		if *slice != nil {
			return nil
		}
		v, err := csup.ReadUint32s(loc, l.r)
		if err != nil {
			return err
		}
		*slice = v
		return nil
	})
}

func (l *loader) loadOffsets(g *errgroup.Group, mu *sync.Mutex, slice *[]uint32, loc csup.Segment, length uint32, nulls *vector.Bool) {
	mu.Lock()
	if *slice != nil {
		mu.Unlock()
		return
	}
	mu.Unlock()
	g.Go(func() error {
		mu.Lock()
		defer mu.Unlock()
		if *slice != nil {
			return nil
		}
		v, err := csup.ReadUint32s(loc, l.r)
		if err != nil {
			return err
		}
		offs := make([]uint32, length+1)
		var off, child uint32
		for k := uint32(0); k < length; k++ {
			offs[k] = off
			if nulls == nil || !nulls.Value(k) {
				off += v[child]
				child++
			}
		}
		offs[length] = off
		*slice = offs
		return nil
	})
}

func (l *loader) fetchNulls(g *errgroup.Group, paths Path, s shadow) {
	switch s := s.(type) {
	case *dynamic:
		for _, m := range s.vals {
			l.fetchNulls(g, paths, m)
		}
	case *record:
		s.nulls.fetch(g, l.r)
		if len(paths) == 0 {
			for _, f := range s.fields {
				l.fetchNulls(g, nil, f.val)
			}
			return
		}
		switch elem := paths[0].(type) {
		case string:
			if k := indexOfField(elem, s.fields); k >= 0 {
				l.fetchNulls(g, paths[1:], s.fields[k].val)
			}
		case Fork:
			for _, path := range elem {
				if name, ok := path[0].(string); ok {
					if k := indexOfField(name, s.fields); k >= 0 {
						l.fetchNulls(g, paths[1:], s.fields[k].val)
					}
				}
			}
		}
	case *array:
		s.nulls.fetch(g, l.r)
		l.fetchNulls(g, paths, s.vals)
	case *set:
		s.nulls.fetch(g, l.r)
		l.fetchNulls(g, paths, s.vals)
	case *map_:
		s.nulls.fetch(g, l.r)
		l.fetchNulls(g, paths, s.keys)
		l.fetchNulls(g, paths, s.vals)
	case *union:
		s.nulls.fetch(g, l.r)
		for _, val := range s.vals {
			l.fetchNulls(g, paths, val)
		}
	case *int_:
		s.nulls.fetch(g, l.r)
	case *uint_:
		s.nulls.fetch(g, l.r)
	case *primitive:
		s.nulls.fetch(g, l.r)
	case *const_:
		s.nulls.fetch(g, l.r)
	case *dict:
		s.nulls.fetch(g, l.r)
	case *error_:
		s.nulls.fetch(g, l.r)
		l.fetchNulls(g, paths, s.vals)
	case *named:
		l.fetchNulls(g, paths, s.vals)
	default:
		panic(fmt.Sprintf("vector cache: type %T not supported", s))
	}
}

func flattenNulls(paths Path, s shadow, parent *vector.Bool) {
	switch s := s.(type) {
	case *dynamic:
		for _, m := range s.vals {
			flattenNulls(paths, m, nil)
		}
	case *record:
		nulls := s.nulls.flatten(parent)
		if len(paths) == 0 {
			for _, f := range s.fields {
				flattenNulls(nil, f.val, nulls)
			}
			return
		}
		switch elem := paths[0].(type) {
		case string:
			if k := indexOfField(elem, s.fields); k >= 0 {
				flattenNulls(paths[1:], s.fields[k].val, nulls)
			}
		case Fork:
			for _, path := range elem {
				if name, ok := path[0].(string); ok {
					if k := indexOfField(name, s.fields); k >= 0 {
						flattenNulls(paths[1:], s.fields[k].val, nulls)
					}
				}
			}
		}
	case *array:
		s.nulls.flatten(parent)
		flattenNulls(paths, s.vals, nil)
	case *set:
		s.nulls.flatten(parent)
		flattenNulls(paths, s.vals, nil)
	case *map_:
		s.nulls.flatten(parent)
		flattenNulls(nil, s.keys, nil)
		flattenNulls(nil, s.vals, nil)
	case *union:
		s.nulls.flatten(parent)
		for _, val := range s.vals {
			flattenNulls(paths, val, nil)
		}
	case *int_:
		s.nulls.flatten(parent)
	case *uint_:
		s.nulls.flatten(parent)
	case *primitive:
		s.nulls.flatten(parent)
	case *const_:
		s.nulls.flatten(parent)
	case *dict:
		s.nulls.flatten(parent)
	case *error_:
		s.nulls.flatten(parent)
		flattenNulls(paths, s.vals, nil)
	case *named:
		flattenNulls(paths, s.vals, parent)
	default:
		panic(fmt.Sprintf("vector cache: type %T not supported", s))
	}
}

func indexOfField(name string, fields []field_) int {
	return slices.IndexFunc(fields, func(f field_) bool {
		return f.name == name
	})
}
