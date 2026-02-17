package expr

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type Dequiet struct {
	sctx    *super.Context
	expr    Evaluator
	builder scode.Builder
}

func NewDequiet(sctx *super.Context, expr Evaluator) Evaluator {
	return &Dequiet{sctx: sctx, expr: expr}
}

func (d *Dequiet) Eval(this super.Value) super.Value {
	val := d.expr.Eval(this)
	if val.Type().Kind() == super.RecordKind {
		d.builder.Reset()
		typ := d.rec(&d.builder, val.Type(), val.Bytes())
		return super.NewValue(typ, d.builder.Bytes().Body())
	}
	return val
}

func (d *Dequiet) rec(builder *scode.Builder, typ super.Type, b scode.Bytes) super.Type {
	rtyp := super.TypeRecordOf(typ)
	if rtyp == nil {
		builder.Append(nil)
		return typ
	}
	var changed bool
	builder.BeginContainer()
	var fields []super.Field
	it := b.Iter()
	for _, f := range rtyp.Fields {
		fbytes := it.Next()
		ftyp := d.dequiet(builder, f.Type, fbytes)
		if ftyp == nil {
			changed = true
			continue
		}
		fields = append(fields, super.NewField(f.Name, ftyp))
	}
	builder.EndContainer()
	if !changed {
		return typ
	}
	return d.sctx.MustLookupTypeRecord(fields)
}

func (d *Dequiet) dequiet(builder *scode.Builder, typ super.Type, b scode.Bytes) super.Type {
	if typ.Kind() == super.RecordKind {
		return d.rec(builder, typ, b)
	}
	if errtyp, ok := typ.(*super.TypeError); ok && errtyp.IsQuiet(b) {
		return nil
	}
	builder.Append(b)
	return typ
}
