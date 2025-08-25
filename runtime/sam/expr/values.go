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
}

func NewRecordExpr(sctx *super.Context, elems []RecordElem) (Evaluator, error) {
	for _, e := range elems {
		if e.Spread != nil {
			return newRecordSpreadExpr(sctx, elems)
		}
	}
	return newRecordExpr(sctx, elems), nil
}

func newRecordExpr(sctx *super.Context, elems []RecordElem) *recordExpr {
	fields := make([]super.Field, 0, len(elems))
	exprs := make([]Evaluator, 0, len(elems))
	for _, elem := range elems {
		fields = append(fields, super.NewField(elem.Name, nil))
		exprs = append(exprs, elem.Field)
	}
	var typ *super.TypeRecord
	if len(exprs) == 0 {
		typ = sctx.MustLookupTypeRecord([]super.Field{})
	}
	return &recordExpr{
		sctx:    sctx,
		typ:     typ,
		builder: scode.NewBuilder(),
		fields:  fields,
		exprs:   exprs,
	}
}

func (r *recordExpr) Eval(this super.Value) super.Value {
	var changed bool
	b := r.builder
	b.Reset()
	for k, e := range r.exprs {
		val := e.Eval(this)
		if r.fields[k].Type != val.Type() {
			r.fields[k].Type = val.Type()
			changed = true
		}
		b.Append(val.Bytes())
	}
	if changed {
		r.typ = r.sctx.MustLookupTypeRecord(r.fields)
	}
	bytes := b.Bytes()
	if bytes == nil {
		// Return empty record instead of null record.
		bytes = []byte{}
	}
	return super.NewValue(r.typ, bytes)
}

type RecordElem struct {
	Name   string
	Field  Evaluator
	Spread Evaluator
}

type recordSpreadExpr struct {
	sctx    *super.Context
	elems   []RecordElem
	builder scode.Builder
	fields  []super.Field
	bytes   []scode.Bytes
	cache   *super.TypeRecord
}

func newRecordSpreadExpr(sctx *super.Context, elems []RecordElem) (*recordSpreadExpr, error) {
	return &recordSpreadExpr{
		sctx:  sctx,
		elems: elems,
	}, nil
}

type fieldValue struct {
	index int
	value super.Value
}

func (r *recordSpreadExpr) Eval(this super.Value) super.Value {
	object := make(map[string]fieldValue)
	for _, elem := range r.elems {
		if elem.Spread != nil {
			rec := elem.Spread.Eval(this)
			if rec.IsMissing() {
				continue
			}
			typ := super.TypeRecordOf(rec.Type())
			if typ == nil {
				// Treat non-record spread values like missing.
				continue
			}
			it := rec.Iter()
			for _, f := range typ.Fields {
				fv, ok := object[f.Name]
				if !ok {
					fv = fieldValue{index: len(object)}
				}
				fv.value = super.NewValue(f.Type, it.Next())
				object[f.Name] = fv
			}
		} else {
			val := elem.Field.Eval(this)
			fv, ok := object[elem.Name]
			if ok {
				fv.value = val
			} else {
				fv = fieldValue{index: len(object), value: val}
			}
			object[elem.Name] = fv
		}
	}
	if len(object) == 0 {
		return super.NewValue(r.sctx.MustLookupTypeRecord([]super.Field{}), []byte{})
	}
	r.update(object)
	b := r.builder
	b.Reset()
	for _, bytes := range r.bytes {
		b.Append(bytes)
	}
	return super.NewValue(r.cache, b.Bytes())
}

// update maps the object into the receiver's vals slice while also
// seeing if we can reuse the cached record type.  If not we look up
// a new type, cache it, and save the field for the cache check.
func (r *recordSpreadExpr) update(object map[string]fieldValue) {
	if len(r.fields) != len(object) {
		r.invalidate(object)
		return
	}
	for name, field := range object {
		if r.fields[field.index] != super.NewField(name, field.value.Type()) {
			r.invalidate(object)
			return
		}
		r.bytes[field.index] = field.value.Bytes()
	}
}

func (r *recordSpreadExpr) invalidate(object map[string]fieldValue) {
	n := len(object)
	r.fields = slices.Grow(r.fields[:0], n)[:n]
	r.bytes = slices.Grow(r.bytes[:0], n)[:n]
	for name, field := range object {
		r.fields[field.index] = super.NewField(name, field.value.Type())
		r.bytes[field.index] = field.value.Bytes()
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
		super.BuildUnion(b, union.TagOf(c.types[0]), c.bytes[0])
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
	unique := types[:0]
	for _, t := range super.UniqueTypes(types) {
		if t != super.TypeNull {
			unique = append(unique, t)
		}
	}
	if len(unique) == 0 {
		return super.TypeNull
	}
	if len(unique) == 1 {
		return unique[0]
	}
	return sctx.LookupTypeUnion(unique)
}
