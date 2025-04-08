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
		panic("vam.DotExpr TypeValue TBD")
	case *vector.Map:
		panic("vam.DotExpr Map TBD")
	case *vector.View:
		return vector.NewView(d.eval(val.Any), val.Index)
	default:
		return vector.NewMissing(d.sctx, val.Len())
	}
}
