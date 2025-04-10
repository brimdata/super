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
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
	"github.com/brimdata/super/zcode"
	"github.com/ronanh/intcomp"
	"golang.org/x/sync/errgroup"
)

// loader handles loading vector data on demand for only the fields needed
// as specified in the projection.  Each load is executed with a multiphase
// process: first, unmarshal builds a mirror of the CSUP metadata where each node has a
// lock and places to store the bulk data so that it may be reused across
// projections.  This is called the shadow object.  The shadow elements are also
// restricted to just the pieces needed for the projection. Then, we fill in the shadow
// with data vectors dynamically and create runtime vectors as follows:
//
//	(1) create a mirror data structure (shadow)
//	(2) concurrently load all the nulls and tags, lens, etc. that will be needed (fetchNulls)
//	(3) compute top-down flattening of nulls concurrently (flatten)
//	(4) load all data that is projected using the nulls to flatten any unloaded data (fetchVals)
//	(5) form a projection from the fully loaded data nodes (project)
//
// The sctx passed into the loader is dynamic and comes from each query context that
// uses the vcache.  No sctx types are stored in the shadow (except for primitive types
// in shadowed vector.Any primitives that are shared).  We otherwise allocate all
// vector.Any super.Types using the passed-in sctx.
type loader struct {
	cctx *csup.Context
	sctx *super.Context
	r    io.ReaderAt
}

// Load all vector data into the in-memory shadow that is needed and not yet loaded
// and return a new vector.Any using the data vectors in cache.  This may be called
// concurrently on the same shadow and fine-grained locking insures that any given
// data vector is loaded just once and such loads may be executed concurrently (even
// when only one thread is calling load).  If paths is nil, then the entire value
// is loaded.  All of the projected paths in the shadow must have been properly
// unmarshaled before calling.
func (l *loader) load(projection field.Projection, s shadow) (vector.Any, error) {
	var group errgroup.Group
	l.fetchNulls(&group, projection, s)
	if err := group.Wait(); err != nil {
		return nil, err
	}
	flattenNulls(projection, s, bitvec.Zero)
	l.loadVector(&group, projection, s)
	if err := group.Wait(); err != nil {
		return nil, err
	}
	return project(l.sctx, projection, s), nil
}

func (l *loader) loadVector(g *errgroup.Group, projection field.Projection, s shadow) {
	switch s := s.(type) {
	case *dynamic:
		//XXX we need an ordered option to load tags only when needed
		l.loadUint32(g, &s.mu, &s.tags, s.meta.Tags)
		for _, m := range s.values {
			if m != nil {
				l.loadVector(g, projection, m)
			}
		}
	case *record:
		l.loadRecord(g, projection, s)
	case *array:
		l.loadOffsets(g, &s.mu, &s.offs, s.meta.Lengths, s.length(), s.nulls.flat)
		l.loadVector(g, projection, s.values)
	case *set:
		l.loadOffsets(g, &s.mu, &s.offs, s.meta.Lengths, s.length(), s.nulls.flat)
		l.loadVector(g, projection, s.values)
	case *map_:
		l.loadOffsets(g, &s.mu, &s.offs, s.meta.Lengths, s.length(), s.nulls.flat)
		l.loadVector(g, projection, s.keys)
		l.loadVector(g, projection, s.values)
	case *union:
		l.loadUnion(g, projection, s)
	case *int_:
		l.loadInt(g, s)
	case *uint_:
		l.loadUint(g, s)
	case *primitive:
		l.loadPrimitive(g, s)
	case *const_:
		s.mu.Lock()
		vec := s.vec
		if vec == nil {
			// Map the const super.Value in the csup's type context to
			// a new one in the query type context.
			val := s.meta.Value
			typ, err := l.sctx.TranslateType(val.Type())
			if err != nil {
				panic(err)
			}
			vec = vector.NewConst(super.NewValue(typ, val.Bytes()), s.length(), s.nulls.flat)
			s.vec = vec
		}
		s.mu.Unlock()
	case *dict:
		l.loadDict(g, projection, s)
	case *error_:
		l.loadVector(g, projection, s.values)
	case *named:
		l.loadVector(g, projection, s.values)
	default:
		panic(fmt.Sprintf("vector cache: shadow type %T not supported", s))
	}
}

func (l *loader) loadRecord(g *errgroup.Group, projection field.Projection, s *record) {
	if len(projection) == 0 {
		// Load the whole record.  We're either loading all on demand (nil paths)
		// or loading this record because it's referenced at the end of a projected path.
		for _, f := range s.fields {
			if f != nil {
				l.loadVector(g, nil, f)
			}
		}
		return
	}
	switch elem := projection[0].(type) {
	case string:
		if k := indexOfField(elem, s.meta); k >= 0 {
			l.loadVector(g, projection[1:], s.fields[k])
		}
	case field.Fork:
		// Multiple fields at this level are being projected.
		for _, path := range elem {
			// records require a field name path element (i.e., string)
			if name, ok := path[0].(string); ok {
				if k := indexOfField(name, s.meta); k >= 0 {
					l.loadVector(g, projection[1:], s.fields[k])
				}
			}
		}
	default:
		panic(fmt.Sprintf("bad path in vcache loadRecord: %T", elem))
	}
}

func (l *loader) loadUnion(g *errgroup.Group, projection field.Projection, s *union) {
	l.loadUint32(g, &s.mu, &s.tags, s.meta.Tags)
	for _, val := range s.values {
		l.loadVector(g, projection, val)
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
		bytes := make([]byte, s.meta.Location.MemLength)
		if err := s.meta.Location.Read(l.r, bytes); err != nil {
			return err
		}
		vals := intcomp.UncompressInt64(byteconv.ReinterpretSlice[uint64](bytes), nil)
		vals = extendForNulls(vals, s.nulls.flat, s.count)
		typ, err := l.sctx.TranslateType(s.meta.Typ)
		if err != nil {
			panic(err)
		}
		s.vec = vector.NewInt(typ, vals, s.nulls.flat)
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
		bytes := make([]byte, s.meta.Location.MemLength)
		if err := s.meta.Location.Read(l.r, bytes); err != nil {
			return err
		}
		vals := intcomp.UncompressUint64(byteconv.ReinterpretSlice[uint64](bytes), nil)
		vals = extendForNulls(vals, s.nulls.flat, s.count)
		typ, err := l.sctx.TranslateType(s.meta.Typ)
		if err != nil {
			panic(err)
		}
		s.vec = vector.NewUint(typ, vals, s.nulls.flat)
		return nil
	})
}

func (l *loader) loadPrimitive(g *errgroup.Group, s *primitive) {
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
		typ, err := l.sctx.TranslateType(s.meta.Typ)
		if err != nil {
			panic(err)
		}
		vec, err := l.loadVals(typ, s, s.nulls.flat)
		if err != nil {
			return err
		}
		s.vec = vec
		return nil
	})
}

func (l *loader) loadVals(typ super.Type, s *primitive, nulls bitvec.Bits) (vector.Any, error) {
	if s.count.vals == 0 {
		// no vals, just nulls
		return empty(typ, s.length(), nulls), nil
	}
	bytes := make([]byte, s.meta.Location.MemLength)
	if err := s.meta.Location.Read(l.r, bytes); err != nil {
		return nil, err
	}
	length := s.length()
	if !nulls.IsZero() && nulls.Len() != length {
		panic(fmt.Sprintf("BAD NULLS LEN nulls %d %d (cnt.vals %d cnt.null %d) %s", nulls.Len(), length, s.count.vals, s.count.nulls, sup.String(typ)))
	}
	it := zcode.Iter(bytes)
	switch typ := typ.(type) {
	case *super.TypeOfUint8, *super.TypeOfUint16, *super.TypeOfUint32, *super.TypeOfUint64:
		values := make([]uint64, length)
		for slot := uint32(0); slot < length; slot++ {
			if !nulls.IsSet(slot) {
				values[slot] = super.DecodeUint(it.Next())
			}
		}
		return vector.NewUint(typ, values, nulls), nil
	case *super.TypeOfInt8, *super.TypeOfInt16, *super.TypeOfInt32, *super.TypeOfInt64, *super.TypeOfDuration, *super.TypeOfTime:
		values := make([]int64, length)
		for slot := uint32(0); slot < length; slot++ {
			if !nulls.IsSet(slot) {
				values[slot] = super.DecodeInt(it.Next())
			}
		}
		return vector.NewInt(typ, values, nulls), nil
	case *super.TypeOfFloat16, *super.TypeOfFloat32, *super.TypeOfFloat64:
		values := make([]float64, length)
		for slot := uint32(0); slot < length; slot++ {
			if !nulls.IsSet(slot) {
				values[slot] = super.DecodeFloat(it.Next())
			}
		}
		return vector.NewFloat(typ, values, nulls), nil
	case *super.TypeOfBool:
		b := vector.NewBoolEmpty(length, nulls)
		for slot := uint32(0); slot < length; slot++ {
			if !nulls.IsSet(slot) {
				if super.DecodeBool(it.Next()) {
					b.Set(slot)
				}
			}
		}
		return b, nil
	case *super.TypeOfBytes:
		bytes := []byte{}
		offs := make([]uint32, length+1)
		var off uint32
		for slot := uint32(0); slot < length; slot++ {
			offs[slot] = off
			if !nulls.IsSet(slot) {
				b := super.DecodeBytes(it.Next())
				bytes = append(bytes, b...)
				off += uint32(len(b))
			}
		}
		offs[length] = off
		return vector.NewBytes(vector.NewBytesTable(offs, bytes), nulls), nil
	case *super.TypeOfString:
		var bytes []byte
		offs := make([]uint32, length+1)
		var off uint32
		for slot := uint32(0); slot < length; slot++ {
			offs[slot] = off
			if !nulls.IsSet(slot) {
				s := super.DecodeString(it.Next())
				bytes = append(bytes, []byte(s)...)
				off += uint32(len(s))
			}
		}
		offs[length] = off
		return vector.NewString(vector.NewBytesTable(offs, bytes), nulls), nil
	case *super.TypeOfIP:
		values := make([]netip.Addr, length)
		for slot := uint32(0); slot < length; slot++ {
			if !nulls.IsSet(slot) {
				values[slot] = super.DecodeIP(it.Next())
			}
		}
		return vector.NewIP(values, nulls), nil
	case *super.TypeOfNet:
		values := make([]netip.Prefix, length)
		for slot := uint32(0); slot < length; slot++ {
			if !nulls.IsSet(slot) {
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
			if !nulls.IsSet(slot) {
				tv := it.Next()
				bytes = append(bytes, tv...)
				off += uint32(len(tv))
			}
		}
		offs[length] = off
		return vector.NewTypeValue(vector.NewBytesTable(offs, bytes), nulls), nil
	case *super.TypeEnum:
		values := make([]uint64, length)
		for slot := range length {
			if !nulls.IsSet(slot) {
				values[slot] = super.DecodeUint(it.Next())
			}
		}
		return vector.NewEnum(typ, values, nulls), nil
	case *super.TypeOfNull:
		return vector.NewConst(super.Null, s.length(), bitvec.Zero), nil
	}
	return nil, fmt.Errorf("internal error: vcache.loadPrimitive got unknown type %#v", typ)
}

func (l *loader) loadDict(g *errgroup.Group, projection field.Projection, s *dict) {
	if s.count.vals == 0 {
		panic("empty dict") // empty dictionaries should not happen!
	}
	l.loadVector(g, projection, s.values)
	l.loadUint32(g, &s.mu, &s.counts, s.meta.Counts)
	g.Go(func() error {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.index = make([]byte, s.meta.Index.MemLength)
		if err := s.meta.Index.Read(l.r, s.index); err != nil {
			return err
		}
		s.index = extendForNulls(s.index, s.nulls.flat, s.count)
		return nil
	})

}

func extendForNulls[T any](in []T, nulls bitvec.Bits, count count) []T {
	if count.nulls == 0 {
		return in
	}
	out := make([]T, count.length())
	var off int
	for i := range count.length() {
		if !nulls.IsSet(i) {
			out[i] = in[off]
			off++
		}
	}
	return out
}

// XXX need nullscnt to pass as length (ugh, need empty buffer nullscnt long because of flattened assumption)
func empty(typ super.Type, length uint32, nulls bitvec.Bits) vector.Any {
	switch typ := typ.(type) {
	case *super.TypeOfUint8, *super.TypeOfUint16, *super.TypeOfUint32, *super.TypeOfUint64:
		return vector.NewUint(typ, make([]uint64, length), nulls)
	case *super.TypeOfInt8, *super.TypeOfInt16, *super.TypeOfInt32, *super.TypeOfInt64, *super.TypeOfDuration, *super.TypeOfTime:
		return vector.NewInt(typ, make([]int64, length), nulls)
	case *super.TypeOfFloat16, *super.TypeOfFloat32, *super.TypeOfFloat64:
		return vector.NewFloat(typ, make([]float64, length), nulls)
	case *super.TypeOfBool:
		return vector.NewBool(bitvec.NewFalse(length), nulls)
	case *super.TypeOfBytes:
		return vector.NewBytes(vector.NewBytesTable(make([]uint32, length+1), nil), nulls)
	case *super.TypeOfString:
		return vector.NewString(vector.NewBytesTable(make([]uint32, length+1), nil), nulls)
	case *super.TypeOfIP:
		return vector.NewIP(make([]netip.Addr, length), nulls)
	case *super.TypeOfNet:
		return vector.NewNet(make([]netip.Prefix, length), nulls)
	case *super.TypeOfType:
		return vector.NewTypeValue(vector.NewBytesTable(make([]uint32, length+1), nil), nulls)
	case *super.TypeOfNull:
		return vector.NewConst(super.Null, length, bitvec.Zero)
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

func (l *loader) loadOffsets(g *errgroup.Group, mu *sync.Mutex, slice *[]uint32, loc csup.Segment, length uint32, nulls bitvec.Bits) {
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
			if !nulls.IsSet(k) {
				off += v[child]
				child++
			}
		}
		offs[length] = off
		*slice = offs
		return nil
	})
}

func (l *loader) fetchNulls(g *errgroup.Group, projection field.Projection, s shadow) {
	switch s := s.(type) {
	case *dynamic:
		for _, m := range s.values {
			l.fetchNulls(g, projection, m)
		}
	case *record:
		s.nulls.fetch(g, l.cctx, l.r)
		if len(projection) == 0 {
			for _, f := range s.fields {
				l.fetchNulls(g, nil, f)
			}
			return
		}
		switch elem := projection[0].(type) {
		case string:
			if k := indexOfField(elem, s.meta); k >= 0 {
				l.fetchNulls(g, projection[1:], s.fields[k])
			}
		case field.Fork:
			for _, path := range elem {
				if name, ok := path[0].(string); ok {
					if k := indexOfField(name, s.meta); k >= 0 {
						l.fetchNulls(g, projection[1:], s.fields[k])
					}
				}
			}
		}
	case *array:
		s.nulls.fetch(g, l.cctx, l.r)
		l.fetchNulls(g, projection, s.values)
	case *set:
		s.nulls.fetch(g, l.cctx, l.r)
		l.fetchNulls(g, projection, s.values)
	case *map_:
		s.nulls.fetch(g, l.cctx, l.r)
		l.fetchNulls(g, projection, s.keys)
		l.fetchNulls(g, projection, s.values)
	case *union:
		s.nulls.fetch(g, l.cctx, l.r)
		for _, val := range s.values {
			l.fetchNulls(g, projection, val)
		}
	case *int_:
		s.nulls.fetch(g, l.cctx, l.r)
	case *uint_:
		s.nulls.fetch(g, l.cctx, l.r)
	case *primitive:
		s.nulls.fetch(g, l.cctx, l.r)
	case *const_:
		s.nulls.fetch(g, l.cctx, l.r)
	case *dict:
		s.nulls.fetch(g, l.cctx, l.r)
	case *error_:
		s.nulls.fetch(g, l.cctx, l.r)
		l.fetchNulls(g, projection, s.values)
	case *named:
		l.fetchNulls(g, projection, s.values)
	default:
		panic(fmt.Sprintf("vector cache: type %T not supported", s))
	}
}

func flattenNulls(projection field.Projection, s shadow, parent bitvec.Bits) {
	switch s := s.(type) {
	case *dynamic:
		for _, m := range s.values {
			flattenNulls(projection, m, bitvec.Zero)
		}
	case *record:
		nulls := s.nulls.flatten(parent)
		if len(projection) == 0 {
			for _, f := range s.fields {
				flattenNulls(nil, f, nulls)
			}
			return
		}
		switch elem := projection[0].(type) {
		case string:
			if k := indexOfField(elem, s.meta); k >= 0 {
				flattenNulls(projection[1:], s.fields[k], nulls)
			}
		case field.Fork:
			for _, path := range elem {
				if name, ok := path[0].(string); ok {
					if k := indexOfField(name, s.meta); k >= 0 {
						flattenNulls(projection[1:], s.fields[k], nulls)
					}
				}
			}
		}
	case *array:
		s.nulls.flatten(parent)
		flattenNulls(projection, s.values, bitvec.Zero)
	case *set:
		s.nulls.flatten(parent)
		flattenNulls(projection, s.values, bitvec.Zero)
	case *map_:
		s.nulls.flatten(parent)
		flattenNulls(nil, s.keys, bitvec.Zero)
		flattenNulls(nil, s.values, bitvec.Zero)
	case *union:
		s.nulls.flatten(parent)
		for _, val := range s.values {
			flattenNulls(projection, val, bitvec.Zero)
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
		flattenNulls(projection, s.values, bitvec.Zero)
	case *named:
		flattenNulls(projection, s.values, parent)
	default:
		panic(fmt.Sprintf("vector cache: type %T not supported", s))
	}
}

func indexOfField(name string, r *csup.Record) int {
	return slices.IndexFunc(r.Fields, func(f csup.Field) bool {
		return f.Name == name
	})
}
