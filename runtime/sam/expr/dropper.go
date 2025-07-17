package expr

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/field"
)

type dropper struct {
	typ       super.Type
	builder   *super.RecordBuilder
	fieldRefs []Evaluator
}

func (d *dropper) drop(in super.Value) super.Value {
	if d.typ == in.Type() {
		return in
	}
	b := d.builder
	b.Reset()
	for _, e := range d.fieldRefs {
		val := e.Eval(in)
		b.Append(val.Bytes())
	}
	val, err := b.Encode()
	if err != nil {
		panic(err)
	}
	return super.NewValue(d.typ, val)
}

type Dropper struct {
	sctx     *super.Context
	fields   field.List
	droppers map[int]*dropper
}

func NewDropper(sctx *super.Context, fields field.List) *Dropper {
	return &Dropper{
		sctx:     sctx,
		fields:   fields,
		droppers: make(map[int]*dropper),
	}
}

func (d *Dropper) newDropper(sctx *super.Context, r super.Value) *dropper {
	fields, fieldTypes, match := complementFields(d.fields, nil, super.TypeRecordOf(r.Type()))
	if !match {
		// r.Type contains no fields matching d.fields, so we set
		// dropper.typ to r.Type to indicate that records of this type
		// should not be modified.
		return &dropper{typ: r.Type()}
	}
	// If the set of dropped fields is equal to the all of record's
	// fields, then there is no output for this input type.
	// We return nil to block this input type.
	if len(fieldTypes) == 0 {
		return nil
	}
	var fieldRefs []Evaluator
	for _, f := range fields {
		fieldRefs = append(fieldRefs, NewDottedExpr(sctx, f))
	}
	builder, err := super.NewRecordBuilder(d.sctx, fields)
	if err != nil {
		panic(err)
	}
	typ := builder.Type(fieldTypes)
	return &dropper{typ, builder, fieldRefs}
}

// complementFields returns the slice of fields and associated types that make
// up the complement of the set of fields in drops along with a boolean that is
// true if typ contains any the fields in drops.
func complementFields(drops field.List, prefix field.Path, typ *super.TypeRecord) (field.List, []super.Type, bool) {
	var fields field.List
	var types []super.Type
	var match bool
	for _, f := range typ.Fields {
		fld := append(prefix, f.Name)
		if drops.Has(fld) {
			match = true
			continue
		}
		if typ, ok := super.TypeUnder(f.Type).(*super.TypeRecord); ok {
			if fs, ts, m := complementFields(drops, fld, typ); m {
				fields = append(fields, fs...)
				types = append(types, ts...)
				match = true
				continue
			}
		}
		fields = append(fields, slices.Clone(fld))
		types = append(types, f.Type)
	}
	return fields, types, match
}

func (d *Dropper) Eval(in super.Value) super.Value {
	if !super.IsRecordType(in.Type()) {
		return in
	}
	id := in.Type().ID()
	dropper, ok := d.droppers[id]
	if !ok {
		dropper = d.newDropper(d.sctx, in)
		d.droppers[id] = dropper
	}
	if dropper == nil {
		return d.sctx.Quiet()
	}
	return dropper.drop(in)
}
