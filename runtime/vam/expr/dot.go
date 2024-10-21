package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
)

type This struct{}

func (*This) Eval(val vector.Any) vector.Any {
	return val
}

type DotExpr struct {
	zctx   *super.Context
	record Evaluator
	field  string
}

func NewDotExpr(zctx *super.Context, record Evaluator, field string) *DotExpr {
	return &DotExpr{
		zctx:   zctx,
		record: record,
		field:  field,
	}
}

func NewDottedExpr(zctx *super.Context, f field.Path) Evaluator {
	ret := Evaluator(&This{})
	for _, name := range f {
		ret = NewDotExpr(zctx, ret, name)
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
			return vector.NewMissing(d.zctx, val.Len())
		}
		return val.Fields[i]
	case *vector.TypeValue:
		panic("vam.DotExpr TypeValue TBD")
	case *vector.Map:
		panic("vam.DotExpr Map TBD")
	default:
		return vector.NewMissing(d.zctx, val.Len())
	}
}
