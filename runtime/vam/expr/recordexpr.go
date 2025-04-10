package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

type RecordElem struct {
	Name string // "" means spread.
	Expr Evaluator
}

func NewRecordExpr(sctx *super.Context, elems []RecordElem) Evaluator {
	return &recordExpr{
		sctx:         sctx,
		elems:        elems,
		fieldIndexes: map[string]int{},
	}
}

type recordExpr struct {
	sctx  *super.Context
	elems []RecordElem

	elemVecs     []vector.Any
	fields       []super.Field
	fieldIndexes map[string]int
	fieldVecs    []vector.Any
}

func (r *recordExpr) Eval(this vector.Any) vector.Any {
	if len(r.elems) == 0 {
		typ := r.sctx.MustLookupTypeRecord(nil)
		return vector.NewRecord(typ, nil, this.Len(), nil)
	}
	r.elemVecs = r.elemVecs[:0]
	for _, elem := range r.elems {
		r.elemVecs = append(r.elemVecs, elem.Expr.Eval(this))
	}
	return vector.Apply(false, r.eval, r.elemVecs...)
}

func (r *recordExpr) eval(vecs ...vector.Any) vector.Any {
	r.fields = r.fields[:0]
	clear(r.fieldIndexes)
	r.fieldVecs = make([]vector.Any, 0, len(r.elems))
	for k, vec := range vecs {
		if name := r.elems[k].Name; name != "" {
			r.addOrUpdateField(name, vec)
		} else {
			r.spread(vec)
		}
	}
	typ := r.sctx.MustLookupTypeRecord(r.fields)
	return vector.NewRecord(typ, r.fieldVecs, r.fieldVecs[0].Len(), nil)
}

func (r *recordExpr) addOrUpdateField(name string, vec vector.Any) {
	if i, ok := r.fieldIndexes[name]; ok {
		r.fields[i].Type = vec.Type()
		r.fieldVecs[i] = vec
		return
	}
	r.fieldIndexes[name] = len(r.fields)
	r.fields = append(r.fields, super.NewField(name, vec.Type()))
	r.fieldVecs = append(r.fieldVecs, vec)
}

func (r *recordExpr) spread(vec vector.Any) {
	// Ignore non-record values.
	switch vec := vector.Under(vec).(type) {
	case *vector.Record:
		for k, f := range super.TypeRecordOf(vec.Type()).Fields {
			r.addOrUpdateField(f.Name, vec.Fields[k])
		}
	case *vector.View:
		if rec, ok := vec.Any.(*vector.Record); ok {
			for k, f := range super.TypeRecordOf(rec.Type()).Fields {
				r.addOrUpdateField(f.Name, vector.Pick(rec.Fields[k], vec.Index))
			}
		}
	}
}
