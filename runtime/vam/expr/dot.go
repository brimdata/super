package expr

import (
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
)

type This struct{}

func (*This) Eval(val vector.Any) vector.Any {
	return val
}

type DotExpr struct {
	sctx    *super.Context
	record  Evaluator
	field   string
	noneish bool
}

func NewDotExpr(sctx *super.Context, record Evaluator, field string, noneish bool) *DotExpr {
	return &DotExpr{
		sctx:    sctx,
		record:  record,
		field:   field,
		noneish: noneish,
	}
}

func NewDottedExpr(sctx *super.Context, f field.Chain) Evaluator {
	ret := Evaluator(&This{})
	for _, elem := range f {
		ret = NewDotExpr(sctx, ret, elem.ID, elem.Noneish)
	}
	return ret
}

func (d *DotExpr) Eval(vec vector.Any) vector.Any {
	return vector.Apply(vector.ApplyRipFusions|vector.ApplyRipUnions, d.eval, d.record.Eval(vec))
}

func (d *DotExpr) eval(vecs ...vector.Any) vector.Any {
	switch val := vector.Under(vector.Super(vecs[0])).(type) {
	case *vector.None:
		return val
	case *vector.Record:
		i, ok := val.Typ.IndexOfField(d.field)
		if !ok {
			if d.noneish {
				return vector.NewNone(val.Len())
			}
			return vector.NewWrappedError(d.sctx, fmt.Sprintf("no such field %s", d.field), val)
		}
		return val.Fields[i]
	case *vector.TypeValue:
		var errs []uint32
		typvals := vector.NewTypeValueEmpty()
		for i := range val.Len() {
			typ := val.Value(i)
			if typ, ok := super.TypeUnder(typ).(*super.TypeRecord); ok {
				if typ, ok := typ.TypeOfField(d.field); ok {
					typvals.Append(typ)
					continue
				}
			}
			errs = append(errs, i)
		}
		if len(errs) > 0 {
			return vector.Combine(typvals, errs, vector.NewMissing(d.sctx, uint32(len(errs))))
		}
		return typvals
	case *vector.Map:
		keyVec := vector.NewConstString(d.field, val.Len())
		return indexMap(d.sctx, val, keyVec)
	case *vector.View:
		return vector.Pick(d.eval(val.Any), val.Index)
	default:
		return vector.NewMissing(d.sctx, val.Len())
	}
}
