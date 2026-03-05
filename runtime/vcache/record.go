package vcache

import (
	"slices"
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
)

type record struct {
	mu     sync.Mutex
	meta   *csup.Record
	len    uint32
	fields []field_
}

type field_ struct {
	meta *csup.Field
	len  uint32
	// values protected by record mutex
	values shadow
	// mu protects nones
	mu     sync.Mutex
	nones  []uint32
	loaded bool
}

func newRecord(cctx *csup.Context, meta *csup.Record) *record {
	fields := make([]field_, len(meta.Fields))
	len := meta.Len(cctx)
	for k := range meta.Fields {
		fields[k].len = len
		fields[k].meta = &meta.Fields[k]
	}
	return &record{
		meta:   meta,
		len:    len,
		fields: fields,
	}
}

func (r *record) length() uint32 {
	return r.len
}

func (r *record) unmarshal(cctx *csup.Context, projection field.Projection) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(projection) == 0 {
		// Unmarshal all the fields of this record.  We're either loading all on demand (nil paths)
		// or loading this record because it's referenced at the end of a projected path.
		for k := range r.fields {
			r.fields[k].unmarshal(cctx, nil)
		}
		return
	}
	for _, node := range projection {
		if k := indexOfField(node.Name, r.meta); k >= 0 {
			r.fields[k].unmarshal(cctx, node.Proj)
		}
	}
}

func (r *record) project(loader *loader, projection field.Projection) vector.Any {
	valFields := make([]vector.Any, 0, len(r.fields))
	types := make([]super.Field, 0, len(r.fields))
	if len(projection) == 0 {
		// Build the whole record.  We're either loading all on demand (nil paths)
		// or loading this record because it's referenced at the end of a projected path.
		for k := range r.fields {
			if r.fields[k].values != nil {
				val := r.fields[k].project(loader, nil)
				valFields = append(valFields, val)
				types = append(types, super.NewFieldWithOpt(r.meta.Fields[k].Name, val.Type(), r.meta.Fields[k].Opt))
			}
		}
		return vector.NewRecord(loader.sctx.MustLookupTypeRecord(types), valFields, r.length())
	}
	fields := make([]super.Field, 0, len(r.fields))
	for _, node := range projection {
		var opt bool
		var val vector.Any
		if k := indexOfField(node.Name, r.meta); k >= 0 && r.fields[k].values != nil {
			val = r.fields[k].project(loader, node.Proj)
			opt = r.meta.Fields[k].Opt
		} else {
			val = vector.NewMissing(loader.sctx, r.length())
		}
		valFields = append(valFields, val)
		fields = append(fields, super.NewFieldWithOpt(node.Name, val.Type(), opt))
	}
	return vector.NewRecord(loader.sctx.MustLookupTypeRecord(fields), valFields, r.length())
}

func indexOfField(name string, r *csup.Record) int {
	return slices.IndexFunc(r.Fields, func(f csup.Field) bool {
		return f.Name == name
	})
}

func (f *field_) unmarshal(cctx *csup.Context, projection field.Projection) {
	// protected by record mutex
	if f.values == nil {
		f.values = newShadow(cctx, f.meta.Values)
	}
	f.values.unmarshal(cctx, projection)
}

func (f *field_) project(loader *loader, projection field.Projection) vector.Any {
	if f.meta.Opt {
		f.mu.Lock()
		if !f.loaded {
			nones, err := csup.ReadUint32s(f.meta.Nones, loader.r)
			if err != nil {
				panic(err)
			}
			f.nones = nones
			f.loaded = true
		}
		f.mu.Unlock()
	}
	return vector.NewFieldFromRLE(loader.sctx, f.values.project(loader, projection), f.len, f.nones)
}
