package jsonvec

import (
	"encoding/binary"
	"strings"

	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

func Materialize(sctx *super.Context, b Builder) vector.Any {
	m := &materializer{
		sctx: sctx,
		defs: super.NewTypeDefs(),
	}
	vec, _ := m.value(b.Value())
	return vec
}

type materializer struct {
	sctx *super.Context
	defs *super.TypeDefs
}

func (m *materializer) value(v Value) (vector.Any, []uint32) {
	switch v := v.(type) {
	case *Null:
		return vector.NewNull(v.len), nil
	case *Bool:
		return vector.NewBool(v.Value), nil
	case *Int:
		return v.Value, nil
	case *Float:
		return v.Value, nil
	case *String:
		return v.Value, nil
	case *Union:
		return m.union(v)
	case *Array:
		return m.array(v)
	case *Record:
		return m.record(v)
	case *Empty:
		// This happens when an array value never saw any contents
		// so all of the inner values of the array are zero length
		// and thus have an unknown inner type.  We return a
		// a zero-length null value here so that the type of the
		// array is [null] as the value will never be used.
		return vector.NewNull(0), nil
	default:
		panic(v)
	}
}

func (m *materializer) union(u *Union) (vector.Any, []uint32) {
	var types []super.Type
	var vecs []vector.Any
	vals := u.Values()
	dynamic := make([][]uint32, 0, len(vals))
	fixed := make([]uint32, 0, len(vals))
	for _, v := range vals {
		vec, ids := m.value(v)
		dynamic = append(dynamic, ids)
		var id uint32
		if ids == nil {
			id = m.defs.LookupType(vec.Type())
		}
		fixed = append(fixed, uint32(id))
		vecs = append(vecs, vec)
		types = append(types, vec.Type())
	}
	utyp, ok := m.sctx.LookupTypeUnion(super.UniqueTypes(types))
	if !ok {
		panic(types)
	}
	subtypes := m.makeUnionSubtypes(u.Tags, dynamic, fixed)
	typ := m.sctx.LookupTypeFusion(utyp)
	loader := &subtypesLoader{
		defs:     m.defs,
		subtypes: subtypes,
	}
	vec := vector.NewUnion(utyp, u.Tags, vecs)
	return vector.NewFusionWithLoader(m.sctx, typ, loader, vec), subtypes
}

func (m *materializer) makeUnionSubtypes(tags []uint32, dynamic [][]uint32, fixed []uint32) []uint32 {
	subtypes := make([]uint32, 0, len(tags))
	for _, tag := range tags {
		var id uint32
		if dynamic[tag] != nil {
			id = dynamic[tag][0]
			dynamic[tag] = dynamic[tag][1:]
		} else {
			id = fixed[tag]
		}
		subtypes = append(subtypes, id)
	}
	return subtypes
}

func (m *materializer) array(a *Array) (vector.Any, []uint32) {
	inner, ids := m.value(a.Inner)
	typ := m.sctx.LookupTypeArray(inner.Type())
	subtypes := m.makeArraySubtypes(ids)
	return vector.NewArray(typ, a.Offsets, inner), subtypes
}

func (m *materializer) makeArraySubtypes(ids []uint32) []uint32 {
	if ids == nil {
		return nil
	}
	subtypes := make([]uint32, 0, len(ids))
	for _, id := range ids {
		subtypes = append(subtypes, m.defs.BindTypeWrapped(super.TypeDefArray, id))
	}
	return subtypes
}

func (m *materializer) record(r *Record) (vector.Any, []uint32) {
	fuseHere := len(r.perm) > 1
	fieldNames := make([]string, len(r.LUT))
	for name, id := range r.LUT {
		fieldNames[id] = name
	}
	n := r.Len()
	var vecs []vector.Any
	var allFields []super.Field
	// dynamic and fixed are index by the supertype's column number.
	// dynamic holds the subtype ID vectors of fields that have them
	// attached (because there is a fusion type below).
	// Otherwise, fixed holds the constant ID for all all rows in that
	// field in each column position.
	dynamic := make([][]uint32, len(r.Fields))
	fixed := make([]uint32, len(r.Fields))
	for i, field := range r.Fields {
		rle := r.RLEs[i].End(n)
		vec, ids := m.value(field.Value)
		dynamic[i] = ids
		// XXX We shouldn't always pollute the typedefs with this
		// type... only if there is a demand for fusion, i.e.,
		// don't put the type ID in the typedefs if it's going to
		// be thrown away?!  This isn't all that big a deal
		// and we can cleanup later.
		if ids == nil {
			fixed[i] = m.defs.LookupType(vec.Type())
		} else {
			fuseHere = true
		}
		field := super.NewFieldWithOpt(fieldNames[i], vec.Type(), len(rle) > 0)
		vecs = append(vecs, vector.NewFieldFromRLE(m.sctx, vec, n, rle))
		allFields = append(allFields, field)
	}
	rtyp := m.sctx.MustLookupTypeRecord(allFields)
	record := vector.NewRecord(rtyp, vecs, n)
	if !fuseHere {
		return record, nil
	}
	subtypes := m.makeRecordSubtypes(r.perm, allFields, dynamic, fixed, r.tags)
	typ := m.sctx.LookupTypeFusion(rtyp)
	loader := &subtypesLoader{
		defs:     m.defs,
		subtypes: subtypes,
	}
	return vector.NewFusionWithLoader(m.sctx, typ, loader, record), subtypes
}

func (m *materializer) makeRecordSubtypes(perm map[string]uint32, fields []super.Field, dynamic [][]uint32, fixed []uint32, tags []uint32) []uint32 {
	// We make a template for each permutation, which is a list of
	// type IDs corresponding to the types of each field.  These
	// will get overwrriten on each lookup to the actual type based
	// on any children subtypes or fixed type ID.  There is also
	// a names slice for each pemutation.
	templates := make([][]int32, len(perm))
	names := make([][]string, len(perm))
	for desc, tag := range perm {
		r := strings.NewReader(desc)
		for {
			col, err := binary.ReadUvarint(r)
			if err != nil {
				break
			}
			templates[tag] = append(templates[tag], int32(col))
			names[tag] = append(names[tag], fields[col].Name)
		}
	}
	// XXX There are a couple optimizations we could do here.
	// First, we could serialize the scatch lookup as the actual
	// typedefs table key directly instead of serializing scratch IDs
	// then making the typedefs key in BindTypeRecord. Second, we
	// could limit the lookups to just each permutation of the
	// record types that is equal to the cardinality of the subtypes
	// slice output.  We will wait on these ideas until profiling
	// suggests otherwise.
	var scratch []uint32
	subtypes := make([]uint32, len(tags))
	for i, tag := range tags {
		scratch = scratch[:0]
		for _, col := range templates[tag] {
			if dynamic[col] != nil {
				scratch = append(scratch, dynamic[col][0])
				dynamic[col] = dynamic[col][1:]
			} else {
				scratch = append(scratch, fixed[col])
			}
		}
		subtypes[i] = m.defs.BindTypeRecord(names[tag], scratch, nil)
	}
	return subtypes
}

type subtypesLoader struct {
	defs     *super.TypeDefs
	subtypes []uint32
}

func (s *subtypesLoader) Load() (*super.TypeDefs, []uint32) {
	return s.defs, s.subtypes
}
