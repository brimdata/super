package vcache

import (
	"fmt"
	"slices"
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type record struct {
	mu   sync.Mutex
	meta *csup.Record
	count
	fields []shadow
	nulls  *nulls
}

func newRecord(cctx *csup.Context, meta *csup.Record, nulls *nulls) *record {
	return &record{
		meta:   meta,
		fields: make([]shadow, len(meta.Fields)),
		nulls:  nulls,
		count:  count{meta.Len(cctx), nulls.count()},
	}
}

func (r *record) unmarshal(cctx *csup.Context, projection field.Projection) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(projection) == 0 {
		// Unmarshal all the fields of this record.  We're either loading all on demand (nil paths)
		// or loading this record because it's referenced at the end of a projected path.
		for k := range r.fields {
			if r.fields[k] == nil {
				r.fields[k] = newShadow(cctx, r.meta.Fields[k].Values, r.nulls)
			}
			r.fields[k].unmarshal(cctx, nil)
		}
		return
	}
	switch elem := projection[0].(type) {
	case string:
		if k := indexOfField(elem, r.meta); k >= 0 {
			if r.fields[k] == nil {
				r.fields[k] = newShadow(cctx, r.meta.Fields[k].Values, r.nulls)
			}
			r.fields[k].unmarshal(cctx, projection[1:])
		}
	case field.Fork:
		// Multiple fields at this level are being projected.
		for _, path := range elem {
			// records require a field name path element (i.e., string)
			if name, ok := path[0].(string); ok {
				if k := indexOfField(name, r.meta); k >= 0 {
					if r.fields[k] == nil {
						r.fields[k] = newShadow(cctx, r.meta.Fields[k].Values, r.nulls)
					}
					r.fields[k].unmarshal(cctx, projection[1:])
				}
			}
		}
	}
}

func (r *record) project(loader *loader, projection field.Projection) vector.Any {
	nulls := r.load(loader)
	if len(projection) == 0 {
		// Build the whole record.  We're either loading all on demand (nil paths)
		// or loading this record because it's referenced at the end of a projected path.
		vecs := make([]vector.Any, 0, len(r.fields))
		types := make([]super.Field, 0, len(r.fields))
		for k, f := range r.fields {
			if f != nil {
				vec := f.project(loader, nil)
				vecs = append(vecs, vec)
				types = append(types, super.Field{Name: r.meta.Fields[k].Name, Type: vec.Type()})
			}
		}
		return vector.NewRecord(loader.sctx.MustLookupTypeRecord(types), vecs, r.length(), nulls)
	}
	switch elem := projection[0].(type) {
	case string:
		// A single path into this vector is projected.
		var vec vector.Any
		if k := indexOfField(elem, r.meta); k >= 0 && r.fields[k] != nil {
			vec = r.fields[k].project(loader, projection[1:])
		} else {
			// Field not here.
			vec = vector.NewMissing(loader.sctx, r.length())
		}
		fields := []super.Field{{Type: vec.Type(), Name: elem}}
		return vector.NewRecord(loader.sctx.MustLookupTypeRecord(fields), []vector.Any{vec}, r.length(), nulls)
	case field.Fork:
		// Multiple paths into this record is projected.  Try to construct
		// each one and slice together the children indicated in the projection.
		vecs := make([]vector.Any, 0, len(r.fields))
		fields := make([]super.Field, 0, len(r.fields))
		for _, path := range elem {
			name := path[0].(string)
			var vec vector.Any
			if k := indexOfField(name, r.meta); k >= 0 && r.fields[k] != nil {
				vec = r.fields[k].project(loader, path[1:])
			} else {
				vec = vector.NewMissing(loader.sctx, r.length())
			}
			vecs = append(vecs, vec)
			fields = append(fields, super.Field{Type: vec.Type(), Name: name})
		}
		return vector.NewRecord(loader.sctx.MustLookupTypeRecord(fields), vecs, r.length(), nulls)
	default:
		panic(fmt.Sprintf("bad path in vcache createRecord: %T", elem))
	}
}

func (r *record) load(loader *loader) bitvec.Bits {
	return r.nulls.get(loader)
}

type recordLoader struct {
	loader *loader
	shadow *record
}

var _ vector.NullsLoader = (*recordLoader)(nil)

func (r *recordLoader) Load() bitvec.Bits {
	return r.shadow.load(r.loader)
}

func indexOfField(name string, r *csup.Record) int {
	return slices.IndexFunc(r.Fields, func(f csup.Field) bool {
		return f.Name == name
	})
}
