package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type This struct{}

func (*This) Eval(val vector.Any) vector.Any {
	return val
}

type DotExpr struct {
	sctx   *super.Context
	record Evaluator
	field  string
}

func NewDotExpr(sctx *super.Context, record Evaluator, field string) *DotExpr {
	return &DotExpr{
		sctx:   sctx,
		record: record,
		field:  field,
	}
}

func NewDottedExpr(sctx *super.Context, f field.Path) Evaluator {
	ret := Evaluator(&This{})
	for _, name := range f {
		ret = NewDotExpr(sctx, ret, name)
	}
	return ret
}

func (d *DotExpr) Eval(vec vector.Any) vector.Any {
	return vector.Apply(true, d.eval, d.record.Eval(vec))
}

func (d *DotExpr) eval(vecs ...vector.Any) vector.Any {
	switch val := vector.Under(vecs[0]).(type) {
	case *vector.Record:
		i, ok := val.Typ.IndexOfField(d.field)
		if !ok {
			return vector.NewMissing(d.sctx, val.Len())
		}
		return val.Fields[i]
	case *vector.TypeValue:
		var errs []uint32
		typvals := vector.NewTypeValueEmpty(0, bitvec.Zero)
		var nulls *vector.Bool
		for i := range val.Len() {
			if val.Nulls.IsSet(i) {
				if nulls == nil {
					nulls = vector.NewBoolEmpty(val.Len(), bitvec.Zero)
				}
				nulls.Set(typvals.Len())
				typvals.Append(nil)
				continue
			}
			typ, _ := d.sctx.DecodeTypeValue(val.Value(i))
			if typ, ok := super.TypeUnder(typ).(*super.TypeRecord); ok {
				if typ, ok := typ.TypeOfField(d.field); ok {
					typvals.Append(super.EncodeTypeValue(typ))
					continue
				}
			}
			errs = append(errs, i)
		}
		if nulls != nil {
			nulls.Bits.Shorten(typvals.Len())
			typvals = vector.CopyAndSetNulls(typvals, nulls.Bits).(*vector.TypeValue)
		}
		if len(errs) > 0 {
			return vector.Combine(typvals, errs, vector.NewMissing(d.sctx, uint32(len(errs))))
		}
		return typvals
	case *vector.Map:
		panic("vam.DotExpr Map TBD")
	case *vector.View:
		return vector.Pick(d.eval(val.Any), val.Index)
	default:
		return vector.NewMissing(d.sctx, val.Len())
	}
}
