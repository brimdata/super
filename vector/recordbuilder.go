package vector

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector/bitvec"
)

type RecordBuilder struct {
	sctx *super.Context
	base *rec
}

func NewRecordBuilder(sctx *super.Context, fields field.List) (*RecordBuilder, error) {
	base := &rec{}
	for _, path := range fields {
		if err := addPath(base, path); err != nil {
			return nil, err
		}
	}
	return &RecordBuilder{sctx: sctx, base: base}, nil
}

func (r *RecordBuilder) New(vecs []Any, nulls bitvec.Bits) *Record {
	rec, _ := r.base.build(r.sctx, vecs)
	rec.Nulls = nulls
	return rec
}

type rec struct {
	paths []string
	recs  []*rec
}

func addPath(r *rec, path field.Path) error {
	for k, name := range path {
		idx := slices.Index(r.paths, name)
		if k == len(path)-1 {
			if idx > -1 {
				return &super.DuplicateFieldError{Name: path.String()}
			}
			r.paths = append(r.paths, path[k])
			r.recs = append(r.recs, nil)
			return nil
		}
		if idx == -1 {
			idx = len(r.paths)
			r.paths = append(r.paths, name)
			r.recs = append(r.recs, &rec{})
		}
		if r.recs[idx] == nil {
			return &super.DuplicateFieldError{Name: path[:k+1].String()}
		}
		r = r.recs[idx]
	}
	return nil
}

func (r *rec) build(sctx *super.Context, leafs []Any) (*Record, []Any) {
	var fields []super.Field
	var out []Any
	for i, name := range r.paths {
		var vec Any
		if r.recs[i] != nil {
			vec, leafs = r.recs[i].build(sctx, leafs)
		} else {
			vec, leafs = leafs[0], leafs[1:]
		}
		fields = append(fields, super.NewField(name, vec.Type()))
		out = append(out, vec)
	}
	typ := sctx.MustLookupTypeRecord(fields)
	return NewRecord(typ, out, out[0].Len(), bitvec.Zero), leafs
}
