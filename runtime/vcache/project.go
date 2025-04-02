package vcache

import (
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
)

func project(sctx *super.Context, projection field.Projection, s shadow) vector.Any {
	switch s := s.(type) {
	case *dynamic:
		return projectDynamic(sctx, projection, s)
	case *record:
		return projectRecord(sctx, projection, s)
	case *array:
		vals := project(sctx, nil, s.values)
		typ := sctx.LookupTypeArray(vals.Type())
		return vector.NewArray(typ, s.offs, vals, s.nulls.flat)
	case *set:
		vals := project(sctx, nil, s.values)
		typ := sctx.LookupTypeSet(vals.Type())
		return vector.NewSet(typ, s.offs, vals, s.nulls.flat)
	case *map_:
		keys := project(sctx, nil, s.keys)
		vals := project(sctx, nil, s.values)
		typ := sctx.LookupTypeMap(keys.Type(), vals.Type())
		return vector.NewMap(typ, s.offs, keys, vals, s.nulls.flat)
	case *union:
		return projectUnion(sctx, nil, s)
	case *int_:
		if len(projection) > 0 {
			return vector.NewMissing(sctx, s.length())
		}
		return s.vec
	case *uint_:
		if len(projection) > 0 {
			return vector.NewMissing(sctx, s.length())
		}
		return s.vec
	case *primitive:
		if len(projection) > 0 {
			return vector.NewMissing(sctx, s.length())
		}
		return s.vec
	case *const_:
		if len(projection) > 0 {
			return vector.NewMissing(sctx, s.length())
		}
		return s.vec
	case *dict:
		if len(projection) > 0 {
			return vector.NewMissing(sctx, s.length())
		}
		vals := project(sctx, projection, s.values)
		return vector.NewDict(vals, s.index, s.counts, s.nulls.flat)
	case *error_:
		v := project(sctx, projection, s.values)
		typ := sctx.LookupTypeError(v.Type())
		return vector.NewError(typ, v, s.nulls.flat)
	case *named:
		v := project(sctx, projection, s.values)
		typ, err := sctx.LookupTypeNamed(s.meta.Name, v.Type())
		if err != nil {
			panic(err)
		}
		return vector.NewNamed(typ, v)
	default:
		panic(fmt.Sprintf("vector cache: shadow type %T not supported", s))
	}
}

func projectDynamic(sctx *super.Context, projection field.Projection, s *dynamic) vector.Any {
	vals := make([]vector.Any, 0, len(s.values))
	for _, m := range s.values {
		vals = append(vals, project(sctx, projection, m))
	}
	return vector.NewDynamic(s.tags, vals)
}

func projectRecord(sctx *super.Context, projection field.Projection, s *record) vector.Any {
	if len(projection) == 0 {
		// Build the whole record.  We're either loading all on demand (nil paths)
		// or loading this record because it's referenced at the end of a projected path.
		vals := make([]vector.Any, 0, len(s.fields))
		types := make([]super.Field, 0, len(s.fields))
		for k, f := range s.fields {
			val := project(sctx, nil, f)
			vals = append(vals, val)
			types = append(types, super.Field{Name: s.meta.Fields[k].Name, Type: val.Type()})
		}
		return vector.NewRecord(sctx.MustLookupTypeRecord(types), vals, s.length(), s.nulls.flat)
	}
	switch elem := projection[0].(type) {
	case string:
		// A single path into this vector is projected.
		var val vector.Any
		if k := indexOfField(elem, s.meta); k >= 0 {
			val = project(sctx, projection[1:], s.fields[k])
		} else {
			// Field not here.
			val = vector.NewMissing(sctx, s.length())
		}
		fields := []super.Field{{Name: elem}}
		return newRecord(sctx, s.length(), fields, []vector.Any{val}, s.nulls.flat)
	case field.Fork:
		// Multiple paths into this record is projected.  Try to construct
		// each one and slice together the children indicated in the projection.
		vals := make([]vector.Any, 0, len(s.fields))
		fields := make([]super.Field, 0, len(s.fields))
		for _, path := range elem {
			//XXX assertion here makes me realize we should have a data structure
			// where a path key is always explicit at the head of a forked path
			name := path[0].(string) // panic if not a string as first elem of fork
			fields = append(fields, super.Field{Name: name})
			if k := indexOfField(name, s.meta); k >= 0 {
				vals = append(vals, project(sctx, path[1:], s.fields[k]))
			} else {
				vals = append(vals, vector.NewMissing(sctx, s.length()))
			}
		}
		return newRecord(sctx, s.length(), fields, vals, s.nulls.flat)
	default:
		panic(fmt.Sprintf("bad path in vcache createRecord: %T", elem))
	}
}

func newRecord(sctx *super.Context, length uint32, fields []super.Field, vals []vector.Any, nulls *vector.Bool) vector.Any {
	for k, val := range vals {
		fields[k].Type = val.Type()
	}
	return vector.NewRecord(sctx.MustLookupTypeRecord(fields), vals, length, nulls)
}

func projectUnion(sctx *super.Context, projection field.Projection, s *union) vector.Any {
	vals := make([]vector.Any, 0, len(s.values))
	types := make([]super.Type, 0, len(s.values))
	for _, val := range s.values {
		val := project(sctx, projection, val)
		vals = append(vals, val)
		types = append(types, val.Type())
	}
	utyp := sctx.LookupTypeUnion(types)
	tags := s.tags
	nulls := s.nulls.flat
	// If there are nulls add a null vector and rebuild tags.
	if nulls != nil {
		var newtags []uint32
		n := uint32(len(vals))
		var nullcount uint32
		for i := range nulls.Len() {
			if nulls.Value(i) {
				newtags = append(newtags, n)
				nullcount++
			} else {
				newtags = append(newtags, tags[0])
				tags = tags[1:]
			}
		}
		tags = newtags
		vals = append(vals, vector.NewConst(super.NewValue(utyp, nil), nullcount, nil))
	}
	return vector.NewUnion(utyp, tags, vals, nulls)
}
