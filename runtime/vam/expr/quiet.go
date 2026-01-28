package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

func QuietMask(vec vector.Any) (vector.Any, bool) {
	errvec, ok := vector.Under(vec).(*vector.Error)
	if !ok || errvec.Vals.Kind() != vector.KindString {
		return vector.NewConst(super.True, vec.Len(), bitvec.Zero), false
	}
	lhs := vector.NewConst(super.NewString("quiet"), vec.Len(), bitvec.Zero)
	out := NewCompare(nil, "!=", nil, nil).Compare(lhs, errvec.Vals)
	if nulls := vector.NullsOf(out); !nulls.IsZero() {
		// Flip nulls to true since a null result is not error("quiet").
		b := FlattenBool(out)
		return vector.NewBool(bitvec.Or(b.Bits, nulls), bitvec.Zero), true
	}
	return out, true
}

type Dequiet struct {
	sctx  *super.Context
	expr  Evaluator
	rmtyp *super.TypeError
}

func NewDequiet(sctx *super.Context, expr Evaluator) Evaluator {
	return &Dequiet{
		sctx:  sctx,
		expr:  expr,
		rmtyp: sctx.LookupTypeError(sctx.MustLookupTypeRecord(nil)),
	}
}

func (d *Dequiet) Eval(this vector.Any) vector.Any {
	return vector.Apply(true, func(vecs ...vector.Any) vector.Any {
		vec := vector.Under(vecs[0])
		if vec.Kind() == vector.KindRecord {
			vec = d.rec(vec)
		}
		return vec
	}, d.expr.Eval(this))
}

func (d *Dequiet) rec(vec vector.Any) vector.Any {
	var index []uint32
	if view, ok := vec.(*vector.View); ok {
		index = view.Index
		vec = view.Any
	}
	var vecs []vector.Any
	rec := vector.Under(vec).(*vector.Record)
	if len(rec.Fields) == 0 {
		return vec
	}
	for _, field := range rec.Fields {
		vec := field
		if index != nil {
			vec = vector.Pick(field, index)
		}
		vecs = append(vecs, d.dequiet(vec))
	}
	if !rec.Nulls.IsZero() {
		// Keep track of incoming nulls
		vecs = append(vecs, vector.NewBool(rec.Nulls, bitvec.Zero))
	}
	return vector.Apply(true, func(vecs ...vector.Any) vector.Any {
		var nulls bitvec.Bits
		if !rec.Nulls.IsZero() {
			nulls = vecs[len(vecs)-1].(*vector.Bool).Bits
			vecs = vecs[:len(vecs)-1]
		}
		var fields []super.Field
		var vals []vector.Any
		for i, vec := range vecs {
			typ := vec.Type()
			if typ == d.rmtyp {
				continue
			}
			fields = append(fields, rec.Typ.Fields[i])
			vals = append(vals, vec)
		}
		rtyp := d.sctx.MustLookupTypeRecord(fields)
		return vector.NewRecord(rtyp, vals, vecs[0].Len(), nulls)
	}, vecs...)
}

func (d *Dequiet) dequiet(vec vector.Any) vector.Any {
	if vec.Kind() == vector.KindRecord {
		return d.rec(vec)
	}
	mask, ok := QuietMask(vec)
	if !ok {
		return vec
	}
	b, _ := BoolMask(new(Not).eval(mask))
	if b.IsEmpty() {
		return vec
	}
	n := uint32(b.GetCardinality())
	quiet := d.quietTmp(n)
	if n == vec.Len() {
		return quiet
	}
	index := b.ToArray()
	vec = vector.ReversePick(vec, index)
	out := vector.Combine(vec, index, quiet).(*vector.Dynamic)
	utyp := d.sctx.LookupTypeUnion([]super.Type{vec.Type(), quiet.Type()})
	return vector.NewUnion(utyp, out.Tags, out.Values, bitvec.Zero)
}

func (d *Dequiet) quietTmp(n uint32) vector.Any {
	return vector.NewError(d.rmtyp, vector.NewConst(super.Null, n, bitvec.Zero), bitvec.Zero)
}
