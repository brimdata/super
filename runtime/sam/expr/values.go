package expr

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type recordExpr struct {
	sctx    *super.Context
	typ     *super.TypeRecord
	builder *scode.Builder
	fields  []super.Field
	exprs   []Evaluator
	nones   []int
}

func NewRecordExpr(sctx *super.Context, elems []RecordElem) Evaluator {
	if evaluator := newRecordExpr(sctx, elems); evaluator != nil {
		return evaluator
	}
	return newRecordSpreadExpr(sctx, elems)
}

func newRecordExpr(sctx *super.Context, elems []RecordElem) *recordExpr {
	fields := make([]super.Field, 0, len(elems))
	exprs := make([]Evaluator, 0, len(elems))
	var optOff int
	var nones []int
	for _, elem := range elems {
		var name string
		var opt bool
		var typ super.Type
		switch elem := elem.(type) {
		case *NoneElem:
			name = elem.Name
			typ = elem.Type
			exprs = append(exprs, nil)
			nones = append(nones, optOff)
			opt = true
		case *FieldElem:
			name = elem.Name
			opt = elem.Opt
			exprs = append(exprs, elem.Expr)
		case *SpreadElem:
			return nil
		}
		fields = append(fields, super.NewFieldWithOpt(name, typ, opt))
		if opt {
			optOff++
		}
	}
	return &recordExpr{
		sctx:    sctx,
		builder: scode.NewBuilder(),
		fields:  fields,
		exprs:   exprs,
		nones:   nones,
	}
}

func (r *recordExpr) Eval(this super.Value) super.Value {
	var changed bool
	b := r.builder
	b.Reset()
	b.BeginContainer()
	for k, e := range r.exprs {
		if e == nil {
			continue
		}
		val := e.Eval(this)
		if r.fields[k].Type != val.Type() {
			r.fields[k].Type = val.Type()
			changed = true
		}
		b.Append(val.Bytes())
	}
	if changed || r.typ == nil {
		r.typ = r.sctx.MustLookupTypeRecord(r.fields)
	}
	b.EndContainerWithNones(r.typ.Opts, r.nones)
	return super.NewValue(r.typ, b.Bytes().Body())
}

type RecordElem interface {
	recordElemSum()
}

type SpreadElem struct {
	Expr Evaluator
}

type FieldElem struct {
	Name string
	Expr Evaluator
	Opt  bool
}

type NoneElem struct {
	Name string
	Type super.Type
}

func (*SpreadElem) recordElemSum() {}
func (*FieldElem) recordElemSum()  {}
func (*NoneElem) recordElemSum()   {}

type recordSpreadExpr struct {
	sctx    *super.Context
	elems   []RecordElem
	builder scode.Builder
	fields  []super.Field
	vals    []fieldValue
	cache   *super.TypeRecord
}

func newRecordSpreadExpr(sctx *super.Context, elems []RecordElem) *recordSpreadExpr {
	return &recordSpreadExpr{
		sctx:  sctx,
		elems: elems,
	}
}

type fieldValue struct {
	index int
	opt   bool
	value super.Value
	none  super.Type
}

func get(rec map[string]fieldValue, name string) fieldValue {
	fv, ok := rec[name]
	if !ok {
		fv = fieldValue{index: len(rec)}
		rec[name] = fv
	}
	return fv
}

func (r *recordSpreadExpr) Eval(this super.Value) super.Value {
	rec := make(map[string]fieldValue)
	for _, elem := range r.elems {
		switch elem := elem.(type) {
		case *SpreadElem:
			val := elem.Expr.Eval(this)
			if val.IsMissing() {
				continue
			}
			typ := super.TypeRecordOf(val.Type())
			if typ == nil {
				// Treat non-record spread values like missing.
				continue
			}
			it := scode.NewRecordIter(val.Bytes(), typ.Opts)
			for _, f := range typ.Fields {
				fv := get(rec, f.Name)
				elem, none := it.Next(f.Opt)
				if none {
					fv.none = f.Type
					fv.opt = true
				} else {
					fv.value = super.NewValue(f.Type, elem)
					fv.opt = f.Opt
					fv.none = nil
				}
				rec[f.Name] = fv
			}
		case *FieldElem:
			val := elem.Expr.Eval(this)
			fv := get(rec, elem.Name)
			fv.value = val
			fv.none = nil
			fv.opt = elem.Opt
			rec[elem.Name] = fv
		case *NoneElem:
			// None
			fv := get(rec, elem.Name)
			fv.none = elem.Type
			fv.opt = true
			rec[elem.Name] = fv
		default:
			panic(elem)
		}
	}
	if len(rec) == 0 {
		return super.NewValue(r.sctx.MustLookupTypeRecord([]super.Field{}), []byte{})
	}
	r.update(rec)
	b := r.builder
	b.Reset()
	b.BeginContainer()
	var optOff int
	var nones []int
	for k, fv := range r.vals {
		if fv.none != nil {
			nones = append(nones, optOff)
			optOff++
			continue
		}
		b.Append(fv.value.Bytes())
		if r.cache.Fields[k].Opt {
			optOff++
		}
	}
	b.EndContainerWithNones(r.cache.Opts, nones)
	return super.NewValue(r.cache, b.Bytes().Body())
}

// update maps the object into the receiver's vals slice while also
// seeing if we can reuse the cached record type.  If not we look up
// a new type, cache it, and save the field for the cache check.
func (r *recordSpreadExpr) update(rec map[string]fieldValue) {
	if len(r.fields) != len(rec) {
		r.invalidate(rec)
		return
	}
	for name, fv := range rec {
		typ := fv.none
		if typ == nil {
			typ = fv.value.Type()
		}
		if r.fields[fv.index] != super.NewFieldWithOpt(name, typ, fv.opt) {
			r.invalidate(rec)
			return
		}
		r.vals[fv.index] = fv
	}
}

func (r *recordSpreadExpr) invalidate(rec map[string]fieldValue) {
	n := len(rec)
	r.fields = slices.Grow(r.fields[:0], n)[:n]
	r.vals = slices.Grow(r.vals[:0], n)[:n]
	for name, fv := range rec {
		typ := fv.none
		if typ == nil {
			typ = fv.value.Type()
		}
		r.fields[fv.index] = super.NewFieldWithOpt(name, typ, fv.opt)
		r.vals[fv.index] = fv
	}
	r.cache = r.sctx.MustLookupTypeRecord(r.fields)
}

type VectorElem struct {
	Value  Evaluator
	Spread Evaluator
}

type ArrayExpr struct {
	elems []VectorElem
	sctx  *super.Context

	builder    scode.Builder
	collection collectionBuilder
}

func NewArrayExpr(sctx *super.Context, elems []VectorElem) *ArrayExpr {
	return &ArrayExpr{
		elems: elems,
		sctx:  sctx,
	}
}

func (a *ArrayExpr) Eval(this super.Value) super.Value {
	a.builder.Reset()
	a.collection.reset()
	for _, e := range a.elems {
		if e.Value != nil {
			a.collection.append(e.Value.Eval(this))
			continue
		}
		val := e.Spread.Eval(this)
		inner := super.InnerType(val.Type())
		if inner == nil {
			// Treat non-list spread values values like missing.
			continue
		}
		a.collection.appendSpread(inner, val.Bytes())
	}
	if len(a.collection.types) == 0 {
		return super.NewValue(a.sctx.LookupTypeArray(super.TypeNull), []byte{})
	}
	it := a.collection.iter(a.sctx)
	for !it.done() {
		it.appendNext(&a.builder)
	}
	return super.NewValue(a.sctx.LookupTypeArray(it.typ), a.builder.Bytes())
}

type SetExpr struct {
	builder    scode.Builder
	collection collectionBuilder
	elems      []VectorElem
	sctx       *super.Context
}

func NewSetExpr(sctx *super.Context, elems []VectorElem) *SetExpr {
	return &SetExpr{
		elems: elems,
		sctx:  sctx,
	}
}

func (a *SetExpr) Eval(this super.Value) super.Value {
	a.builder.Reset()
	a.collection.reset()
	for _, e := range a.elems {
		if e.Value != nil {
			a.collection.append(e.Value.Eval(this))
			continue
		}
		val := e.Spread.Eval(this)
		inner := super.InnerType(val.Type())
		if inner == nil {
			// Treat non-list spread values values like missing.
			continue
		}
		a.collection.appendSpread(inner, val.Bytes())
	}
	if len(a.collection.types) == 0 {
		return super.NewValue(a.sctx.LookupTypeSet(super.TypeNull), []byte{})
	}
	it := a.collection.iter(a.sctx)
	for !it.done() {
		it.appendNext(&a.builder)
	}
	return super.NewValue(a.sctx.LookupTypeSet(it.typ), super.NormalizeSet(a.builder.Bytes()))
}

type Entry struct {
	Key Evaluator
	Val Evaluator
}

type MapExpr struct {
	builder scode.Builder
	entries []Entry
	keys    collectionBuilder
	vals    collectionBuilder
	sctx    *super.Context
}

func NewMapExpr(sctx *super.Context, entries []Entry) *MapExpr {
	return &MapExpr{
		entries: entries,
		sctx:    sctx,
	}
}

func (m *MapExpr) Eval(this super.Value) super.Value {
	m.keys.reset()
	m.vals.reset()
	for _, e := range m.entries {
		m.keys.append(e.Key.Eval(this))
		m.vals.append(e.Val.Eval(this))
	}
	if len(m.keys.types) == 0 {
		typ := m.sctx.LookupTypeMap(super.TypeNull, super.TypeNull)
		return super.NewValue(typ, []byte{})
	}
	m.builder.Reset()
	kIter, vIter := m.keys.iter(m.sctx), m.vals.iter(m.sctx)
	for !kIter.done() {
		kIter.appendNext(&m.builder)
		vIter.appendNext(&m.builder)
	}
	bytes := m.builder.Bytes()
	typ := m.sctx.LookupTypeMap(kIter.typ, vIter.typ)
	return super.NewValue(typ, super.NormalizeMap(bytes))
}

type collectionBuilder struct {
	types       []super.Type
	uniqueTypes []super.Type
	bytes       []scode.Bytes
}

func (c *collectionBuilder) reset() {
	c.types = c.types[:0]
	c.uniqueTypes = c.uniqueTypes[:0]
	c.bytes = c.bytes[:0]
}

func (c *collectionBuilder) append(val super.Value) {
	c.types = append(c.types, val.Type())
	c.bytes = append(c.bytes, val.Bytes())
}

func (c *collectionBuilder) appendSpread(inner super.Type, b scode.Bytes) {
	union, _ := super.TypeUnder(inner).(*super.TypeUnion)
	for it := b.Iter(); !it.Done(); {
		typ := inner
		bytes := it.Next()
		if union != nil {
			typ, bytes = union.Untag(bytes)
		}
		c.types = append(c.types, typ)
		c.bytes = append(c.bytes, bytes)
	}
}

func (c *collectionBuilder) iter(sctx *super.Context) collectionIter {
	// uniqueTypes must be copied since super.UniqueTypes operates on the type
	// array in place and thus we'll lose order.
	c.uniqueTypes = append(c.uniqueTypes[:0], c.types...)
	return collectionIter{
		typ:   unionOf(sctx, c.uniqueTypes),
		bytes: c.bytes,
		types: c.types,
		uniq:  len(c.uniqueTypes),
	}
}

type collectionIter struct {
	typ   super.Type
	bytes []scode.Bytes
	types []super.Type
	uniq  int
}

func (c *collectionIter) appendNext(b *scode.Builder) {
	if union, ok := c.typ.(*super.TypeUnion); ok && c.uniq > 1 {
		val := super.NewValue(c.types[0], c.bytes[0]).Deunion()
		super.BuildUnion(b, union.TagOf(val.Type()), val.Bytes())
	} else {
		b.Append(c.bytes[0])
	}
	c.bytes = c.bytes[1:]
	c.types = c.types[1:]
}

func (c *collectionIter) done() bool {
	return len(c.types) == 0
}

func unionOf(sctx *super.Context, types []super.Type) super.Type {
	if len(types) == 0 {
		return super.TypeNull
	}
	unique := super.Flatten(super.UniqueTypes(types))
	if len(unique) == 1 {
		return unique[0]
	}
	out, _ := sctx.LookupTypeUnion(unique)
	return out
}
