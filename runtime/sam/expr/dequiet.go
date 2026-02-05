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
	if b == nil { //XXX?
		builder.Append(nil)
		return typ
	}
	rtyp := super.TypeRecordOf(typ)
	if rtyp == nil {
		panic(typ)
	}
	var changed bool
	builder.BeginContainer()
	var fields []super.Field
	it := scode.NewRecordIter(b, rtyp.Opts)
	// For building the output record, we don't how many optional fields there
	// will be until after we make the type.  So we call EndContainerWithBits
	// to get deal with.
	var nones []int
	var optOff int
	for _, f := range rtyp.Fields {
		//XXX need to handle none, which is never quiet
		// (need to update the none bits which may move due to dequieting
		// and put them at the front)
		fbytes, none := it.Next(f.Opt)
		if none {
			nones = append(nones, optOff)
			fields = append(fields, super.NewField(f.Name, f.Type, f.Opt))
			optOff++
			continue
		}
		ftyp := d.dequiet(builder, f.Type, fbytes)
		if ftyp == nil {
			changed = true
			continue
		}
		fields = append(fields, super.NewField(f.Name, ftyp, f.Opt))
		if f.Opt {
			optOff++
		}
	}
	if changed {
		rtyp = d.sctx.MustLookupTypeRecord(fields)
		typ = rtyp
	}
	builder.EndContainerWithNones(rtyp.Opts, nones)
	return typ
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
