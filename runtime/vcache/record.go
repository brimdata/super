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
	vecs := make([]vector.Any, 0, len(r.fields))
	types := make([]super.Field, 0, len(r.fields))
	if len(projection) == 0 {
		// Build the whole record.  We're either loading all on demand (nil paths)
		// or loading this record because it's referenced at the end of a projected path.
		for k := range r.fields {
			if r.fields[k].values != nil { //XXX why this nil check?
				vec := r.fields[k].values.project(loader, nil)
				vecs = append(vecs, vec)
				types = append(types, super.NewField(r.meta.Fields[k].Name, vec.Type(), r.meta.Fields[k].Opt))
			}
		}
		return vector.NewRecord(loader.sctx.MustLookupTypeRecord(types), vecs, r.length())
	}
	fields := make([]super.Field, 0, len(r.fields))
	for _, node := range projection {
		var vec vector.Any
		var opt bool
		if k := indexOfField(node.Name, r.meta); k >= 0 && r.fields[k].values != nil {
			vec = r.fields[k].values.project(loader, node.Proj)
			opt = r.meta.Fields[k].Opt
		} else {
			vec = vector.NewMissing(loader.sctx, r.length())
		}
		vecs = append(vecs, vec)
		fields = append(fields, super.NewField(node.Name, vec.Type(), opt))
	}
	return vector.NewRecord(loader.sctx.MustLookupTypeRecord(fields), vecs, r.length())
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
	vals := f.values.project(loader, projection)
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
		if len(f.nones) != 0 {
			return f.dynamic(loader.sctx, vals)
		}
	}
	return vals
}

// XXX for now we go ahead and make a regular vector.Dynamic here.  This is going
// to be inefficient when there are lots of fused chunks and/or when the dynamic is
// not needed (because the query filters the nones).  We should have a more efficient
// design of something that lives in package vector and in vam that lazily loads
// the nones annd computes a Dynamic only when needed.
// XXX we need to change this for summit.
func (f *field_) dynamic(sctx *super.Context, vals vector.Any) vector.Any {
	tags, noneLen := f.buildTags(f.len)
	if noneLen == 0 {
		return vals
	}
	errs := vector.NewMissing(sctx, f.len-noneLen)
	return vector.NewDynamic(tags, []vector.Any{vals, errs})
}

func (f *field_) buildTags(n uint32) ([]uint32, uint32) {
	tags := make([]uint32, n)
	off := 0
	var noneLen uint32
	for in := 0; in < len(f.nones); {
		noneRun := f.nones[in]
		in++
		for k := range int(noneRun) {
			tags[off+k] = 1
		}
		off += int(noneRun)
		noneLen += noneRun
		if in >= len(f.nones) {
			break
		}
		// skip over values (leaving tags 0)
		off += int(f.nones[in])
		in++
	}
	return tags, noneLen
}
