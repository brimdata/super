package vcache

import (
	"fmt"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/vector"
)

func project(zctx *zed.Context, paths Path, s shadow) vector.Any {
	switch s := s.(type) {
	case *dynamic:
		return projectDynamic(zctx, paths, s)
	case *record:
		return projectRecord(zctx, paths, s)
	case *array:
		vals := project(zctx, nil, s.vals)
		typ := zctx.LookupTypeArray(vals.Type())
		return vector.NewArray(typ, s.offs, vals, s.nulls.flat)
	case *set:
		vals := project(zctx, nil, s.vals)
		typ := zctx.LookupTypeSet(vals.Type())
		return vector.NewSet(typ, s.offs, vals, s.nulls.flat)
	case *map_:
		keys := project(zctx, nil, s.keys)
		vals := project(zctx, nil, s.vals)
		typ := zctx.LookupTypeMap(keys.Type(), vals.Type())
		return vector.NewMap(typ, s.offs, keys, vals, s.nulls.flat)
	case *union:
		return projectUnion(zctx, nil, s)
	case *primitive:
		if len(paths) > 0 {
			return vector.NewMissing(zctx, s.length())
		}
		return s.vec
	case *const_:
		if len(paths) > 0 {
			return vector.NewMissing(zctx, s.length())
		}
		return s.vec
	case *error_:
		v := project(zctx, paths, s.vals)
		typ := zctx.LookupTypeError(v.Type())
		return vector.NewError(typ, v, s.nulls.flat)
	case *named:
		v := project(zctx, paths, s.vals)
		typ, err := zctx.LookupTypeNamed(s.name, v.Type())
		if err != nil {
			panic(err)
		}
		return vector.NewNamed(typ, v)
	default:
		panic(fmt.Sprintf("vector cache: shadow type %T not supported", s))
	}
}

func projectDynamic(zctx *zed.Context, paths Path, s *dynamic) vector.Any {
	vals := make([]vector.Any, 0, len(s.vals))
	for _, m := range s.vals {
		vals = append(vals, project(zctx, paths, m))
	}
	return vector.NewDynamic(s.tags, vals)
}

func projectRecord(zctx *zed.Context, paths Path, s *record) vector.Any {
	if len(paths) == 0 {
		// Build the whole record.  We're either loading all on demand (nil paths)
		// or loading this record because it's referenced at the end of a projected path.
		vals := make([]vector.Any, 0, len(s.fields))
		types := make([]zed.Field, 0, len(s.fields))
		for _, f := range s.fields {
			val := project(zctx, nil, f.val)
			vals = append(vals, val)
			types = append(types, zed.Field{Name: f.name, Type: val.Type()})
		}
		return vector.NewRecord(zctx.MustLookupTypeRecord(types), vals, s.length(), s.nulls.flat)
	}
	switch elem := paths[0].(type) {
	case string:
		// A single path into this vector is projected.
		var val vector.Any
		if k := indexOfField(elem, s.fields); k >= 0 {
			val = project(zctx, paths[1:], s.fields[k].val)
		} else {
			// Field not here.
			val = vector.NewMissing(zctx, s.length())
		}
		fields := []zed.Field{{Name: elem}}
		return newRecord(zctx, s.length(), fields, []vector.Any{val}, s.nulls.flat)
	case Fork:
		// Multiple paths into this record is projected.  Try to construct
		// each one and slice together the children indicated in the projection.
		vals := make([]vector.Any, 0, len(s.fields))
		fields := make([]zed.Field, 0, len(s.fields))
		for _, path := range elem {
			//XXX assertion here makes me realize we should have a data structure
			// where a path key is always explicit at the head of a forked path
			name := path[0].(string) // panic if not a string as first elem of fork
			fields = append(fields, zed.Field{Name: name})
			if k := indexOfField(name, s.fields); k >= 0 {
				vals = append(vals, project(zctx, path[1:], s.fields[k].val))
			} else {
				vals = append(vals, vector.NewMissing(zctx, s.length()))
			}
		}
		return newRecord(zctx, s.length(), fields, vals, s.nulls.flat)
	default:
		panic(fmt.Sprintf("bad path in vcache createRecord: %T", elem))
	}
}

func newRecord(zctx *zed.Context, length uint32, fields []zed.Field, vals []vector.Any, nulls *vector.Bool) vector.Any {
	for k, val := range vals {
		fields[k].Type = val.Type()
	}
	return vector.NewRecord(zctx.MustLookupTypeRecord(fields), vals, length, nulls)
}

func projectUnion(zctx *zed.Context, paths Path, s *union) vector.Any {
	vals := make([]vector.Any, 0, len(s.vals))
	types := make([]zed.Type, 0, len(s.vals))
	for _, val := range s.vals {
		val := project(zctx, paths, val)
		vals = append(vals, val)
		types = append(types, val.Type())
	}
	return vector.NewUnion(zctx.LookupTypeUnion(types), s.tags, vals, s.nulls.flat)
}
