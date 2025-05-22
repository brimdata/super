package vcache

import (
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
	for _, node := range projection {
		if k := indexOfField(node.Name, r.meta); k >= 0 {
			if r.fields[k] == nil {
				r.fields[k] = newShadow(cctx, r.meta.Fields[k].Values, r.nulls)
			}
			r.fields[k].unmarshal(cctx, node.Proj)
		}
	}
}

func (r *record) project(loader *loader, projection field.Projection) vector.Any {
	nulls := r.load(loader)
	vecs := make([]vector.Any, 0, len(r.fields))
	types := make([]super.Field, 0, len(r.fields))
	if len(projection) == 0 {
		// Build the whole record.  We're either loading all on demand (nil paths)
		// or loading this record because it's referenced at the end of a projected path.
		for k, f := range r.fields {
			if f != nil {
				vec := f.project(loader, nil)
				vecs = append(vecs, vec)
				types = append(types, super.NewField(r.meta.Fields[k].Name, vec.Type()))
			}
		}
		return vector.NewRecord(loader.sctx.MustLookupTypeRecord(types), vecs, r.length(), nulls)
	}
	fields := make([]super.Field, 0, len(r.fields))
	for _, node := range projection {
		var vec vector.Any
		if k := indexOfField(node.Name, r.meta); k >= 0 && r.fields[k] != nil {
			vec = r.fields[k].project(loader, node.Proj)
		} else {
			vec = vector.NewMissing(loader.sctx, r.length())
		}
		vecs = append(vecs, vec)
		fields = append(fields, super.NewField(node.Name, vec.Type()))
	}
	return vector.NewRecord(loader.sctx.MustLookupTypeRecord(fields), vecs, r.length(), nulls)
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
