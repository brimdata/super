package expr

import (
	"fmt"
	"slices"
	"sort"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zcode"
)

// A ShaperTransform represents one of the different transforms that a
// shaper can apply.  The transforms are represented as bit flags that
// can be bitwise-ored together to create a single shaping operator that
// represents the composition of all operators.  This composition is efficient
// as it is created once per incoming type and then the resulting
// operator is run for every value of that type.
type ShaperTransform int

const (
	Cast ShaperTransform = 1 << iota
	Crop
	Fill
	Order
)

func NewShaperTransform(s string) ShaperTransform {
	switch s {
	case "cast":
		return Cast
	case "crop":
		return Crop
	case "fill":
		return Fill
	case "fit":
		return Crop | Fill
	case "order":
		return Order
	case "shape":
		return Cast | Fill | Order
	}
	return 0
}

// NewShaper returns a shaper that will shape the result of expr
// to the type returned by typeExpr according to tf.
func NewShaper(sctx *super.Context, expr, typeExpr Evaluator, tf ShaperTransform) (Evaluator, error) {
	if l, ok := typeExpr.(*Literal); ok {
		typeVal := l.val
		switch id := typeVal.Type().ID(); {
		case id == super.IDType:
			typ, err := sctx.LookupByValue(typeVal.Bytes())
			if err != nil {
				return nil, err
			}
			return NewConstShaper(sctx, expr, typ, tf), nil
		case id == super.IDString && tf == Cast:
			name := super.DecodeString(typeVal.Bytes())
			if _, err := super.NewContext().LookupTypeNamed(name, super.TypeNull); err != nil {
				return nil, err
			}
			return &casterNamedType{sctx, expr, name}, nil
		}
		return nil, fmt.Errorf("shaper type argument is not a type: %s", sup.FormatValue(typeVal))
	}
	return &Shaper{
		sctx:       sctx,
		expr:       expr,
		typeExpr:   typeExpr,
		transforms: tf,
		shapers:    make(map[super.Type]*ConstShaper),
	}, nil
}

type Shaper struct {
	sctx       *super.Context
	expr       Evaluator
	typeExpr   Evaluator
	transforms ShaperTransform

	shapers map[super.Type]*ConstShaper
}

func (s *Shaper) Eval(this super.Value) super.Value {
	typeVal := s.typeExpr.Eval(this)
	switch id := typeVal.Type().ID(); {
	case id == super.IDType:
		typ, err := s.sctx.LookupByValue(typeVal.Bytes())
		if err != nil {
			return s.sctx.NewError(err)
		}
		shaper, ok := s.shapers[typ]
		if !ok {
			shaper = NewConstShaper(s.sctx, s.expr, typ, s.transforms)
			s.shapers[typ] = shaper
		}
		return shaper.Eval(this)
	case id == super.IDString && s.transforms == Cast:
		name := super.DecodeString(typeVal.Bytes())
		return (&casterNamedType{s.sctx, s.expr, name}).Eval(this)
	}
	return s.sctx.WrapError("shaper type argument is not a type", typeVal)
}

type ConstShaper struct {
	sctx       *super.Context
	expr       Evaluator
	shapeTo    super.Type
	transforms ShaperTransform

	b       zcode.Builder
	caster  Evaluator       // used when shapeTo is a primitive type
	shapers map[int]*shaper // map from input type ID to shaper
}

// NewConstShaper returns a shaper that will shape the result of expr
// to the provided shapeTo type.
func NewConstShaper(sctx *super.Context, expr Evaluator, shapeTo super.Type, tf ShaperTransform) *ConstShaper {
	var caster Evaluator
	if tf == Cast {
		// Use a caster since it's faster.
		caster = LookupPrimitiveCaster(sctx, super.TypeUnder(shapeTo))
	}
	return &ConstShaper{
		sctx:       sctx,
		expr:       expr,
		shapeTo:    shapeTo,
		transforms: tf,
		caster:     caster,
		shapers:    make(map[int]*shaper),
	}
}

func (c *ConstShaper) Eval(this super.Value) super.Value {
	val := c.expr.Eval(this)
	if val.IsError() {
		return val
	}
	if val.IsNull() {
		// Null values can be shaped to any type.
		return super.NewValue(c.shapeTo, nil)
	}
	id, shapeToID := val.Type().ID(), c.shapeTo.ID()
	if id == shapeToID {
		// Same underlying types but one or both are named.
		return super.NewValue(c.shapeTo, val.Bytes())
	}
	if c.caster != nil && !super.IsUnionType(val.Type()) {
		val = c.caster.Eval(val)
		if val.Type() != c.shapeTo && val.Type().ID() == shapeToID {
			// Same underlying types but one or both are named.
			return super.NewValue(c.shapeTo, val.Bytes())
		}
		return val
	}
	s, ok := c.shapers[id]
	if !ok {
		var err error
		s, err = newShaper(c.sctx, c.transforms, val.Type(), c.shapeTo)
		if err != nil {
			return c.sctx.NewError(err)
		}
		c.shapers[id] = s
	}
	c.b.Reset()
	typ := s.step.build(c.sctx, val.Bytes(), &c.b)
	return super.NewValue(typ, c.b.Bytes().Body())
}

// A shaper is a per-input type ID "spec" that contains the output
// type and the op to create an output value.
type shaper struct {
	typ  super.Type
	step step
}

func newShaper(sctx *super.Context, tf ShaperTransform, in, out super.Type) (*shaper, error) {
	typ, err := shaperType(sctx, tf, in, out)
	if err != nil {
		return nil, err
	}
	step, err := newStep(sctx, in, typ)
	return &shaper{typ, step}, err
}

func shaperType(sctx *super.Context, tf ShaperTransform, in, out super.Type) (super.Type, error) {
	inUnder, outUnder := super.TypeUnder(in), super.TypeUnder(out)
	if tf&Cast != 0 {
		if inUnder == outUnder || inUnder == super.TypeNull {
			return out, nil
		}
		if isMap(outUnder) {
			return nil, fmt.Errorf("cannot yet use maps in shaping functions (issue #2894)")
		}
		if super.IsPrimitiveType(inUnder) && super.IsPrimitiveType(outUnder) {
			// Matching field is a primitive: output type is cast type.
			if LookupPrimitiveCaster(sctx, outUnder) == nil {
				return nil, fmt.Errorf("cast to %s not implemented", sup.FormatType(out))
			}
			return out, nil
		}
		if in, ok := inUnder.(*super.TypeUnion); ok {
			for _, t := range in.Types {
				if _, err := shaperType(sctx, tf, t, out); err != nil {
					return nil, fmt.Errorf("cannot cast union %q to %q due to %q",
						sup.FormatType(in), sup.FormatType(out), sup.FormatType(t))
				}
			}
			return out, nil
		}
		if bestUnionTag(in, outUnder) > -1 {
			return out, nil
		}
	} else if inUnder == outUnder {
		return in, nil
	}
	if inRec, ok := inUnder.(*super.TypeRecord); ok {
		if outRec, ok := outUnder.(*super.TypeRecord); ok {
			fields, err := shaperFields(sctx, tf, inRec, outRec)
			if err != nil {
				return nil, err
			}
			if tf&Cast != 0 {
				if slices.Equal(fields, outRec.Fields) {
					return out, nil
				}
			} else if slices.Equal(fields, inRec.Fields) {
				return in, nil
			}
			return sctx.LookupTypeRecord(fields)
		}
	}
	inInner, outInner := super.InnerType(inUnder), super.InnerType(outUnder)
	if inInner != nil && outInner != nil && (tf&Cast != 0 || isArray(inUnder) == isArray(outUnder)) {
		t, err := shaperType(sctx, tf, inInner, outInner)
		if err != nil {
			return nil, err
		}
		if tf&Cast != 0 {
			if t == outInner {
				return out, nil
			}
		} else if t == inInner {
			return in, nil
		}
		if isArray(outUnder) {
			return sctx.LookupTypeArray(t), nil
		}
		return sctx.LookupTypeSet(t), nil
	}
	return in, nil
}

func shaperFields(sctx *super.Context, tf ShaperTransform, in, out *super.TypeRecord) ([]super.Field, error) {
	crop, fill := tf&Crop != 0, tf&Fill != 0
	if tf&Order == 0 {
		crop, fill = !fill, !crop
		out, in = in, out
	}
	var fields []super.Field
	for _, outField := range out.Fields {
		if inFieldType, ok := in.TypeOfField(outField.Name); ok {
			outFieldType := outField.Type
			if tf&Order == 0 {
				// Counteract the swap of in and out above.
				outFieldType, inFieldType = inFieldType, outFieldType
			}
			t, err := shaperType(sctx, tf, inFieldType, outFieldType)
			if err != nil {
				return nil, err
			}
			fields = append(fields, super.NewField(outField.Name, t))
		} else if fill {
			fields = append(fields, outField)
		}
	}
	if !crop {
		inFields := in.Fields
		if tf&Order != 0 {
			// Order appends unknown fields in lexicographic order.
			inFields = slices.Clone(inFields)
			sort.Slice(inFields, func(i, j int) bool {
				return inFields[i].Name < inFields[j].Name
			})
		}
		for _, f := range inFields {
			if !out.HasField(f.Name) {
				fields = append(fields, f)
			}
		}
	}
	return fields, nil
}

// bestUnionTag tries to return the most specific union tag for in
// within out.  It returns -1 if out is not a union or contains no type
// compatible with in.  (Types are compatible if they have the same underlying
// type.)  If out contains in, bestUnionTag returns its tag.
// Otherwise, if out contains in's underlying type, bestUnionTag returns
// its tag.  Finally, bestUnionTag returns the smallest tag in
// out whose type is compatible with in.
func bestUnionTag(in, out super.Type) int {
	outUnion, ok := super.TypeUnder(out).(*super.TypeUnion)
	if !ok {
		return -1
	}
	typeUnderIn := super.TypeUnder(in)
	underlying := -1
	compatible := -1
	for i, t := range outUnion.Types {
		if t == in {
			return i
		}
		if t == typeUnderIn && underlying == -1 {
			underlying = i
		}
		if super.TypeUnder(t) == typeUnderIn && compatible == -1 {
			compatible = i
		}
	}
	if underlying != -1 {
		return underlying
	}
	return compatible
}

func isArray(t super.Type) bool {
	_, ok := t.(*super.TypeArray)
	return ok
}

func isMap(t super.Type) bool {
	_, ok := t.(*super.TypeMap)
	return ok
}

type op int

const (
	copyOp        op = iota // copy field fromIndex from input record
	castPrimitive           // cast field fromIndex from fromType to toType
	castFromUnion           // cast union value with tag s using children[s]
	castToUnion             // cast non-union fromType to union toType with tag toTag
	null                    // write null
	array                   // build array
	set                     // build set
	record                  // build record
)

// A step is a recursive data structure encoding a series of
// copy/cast steps to be carried out over an input record.
type step struct {
	op        op
	caster    Evaluator  // for castPrimitive
	fromIndex int        // for children of a record step
	fromType  super.Type // for castPrimitive and castToUnion
	toTag     int        // for castToUnion
	toType    super.Type
	// if op == record, contains one op for each field.
	// if op == array, contains one op for all array elements.
	// if op == castFromUnion, contains one op per union tag.
	children []step

	types       []super.Type
	uniqueTypes []super.Type
}

func newStep(sctx *super.Context, in, out super.Type) (step, error) {
Switch:
	switch {
	case in.ID() == super.IDNull:
		return step{op: null, toType: out}, nil
	case in.ID() == out.ID():
		return step{op: copyOp, toType: out}, nil
	case super.IsRecordType(in) && super.IsRecordType(out):
		return newRecordStep(sctx, super.TypeRecordOf(in), out)
	case super.IsPrimitiveType(in) && super.IsPrimitiveType(out):
		caster := LookupPrimitiveCaster(sctx, super.TypeUnder(out))
		return step{op: castPrimitive, caster: caster, fromType: in, toType: out}, nil
	case super.InnerType(in) != nil:
		if k := out.Kind(); k == super.ArrayKind {
			return newArrayOrSetStep(sctx, array, super.InnerType(in), out)
		} else if k == super.SetKind {
			return newArrayOrSetStep(sctx, set, super.InnerType(in), out)
		}
	case super.IsUnionType(in):
		var steps []step
		for _, t := range super.TypeUnder(in).(*super.TypeUnion).Types {
			s, err := newStep(sctx, t, out)
			if err != nil {
				break Switch
			}
			steps = append(steps, s)
		}
		return step{op: castFromUnion, toType: out, children: steps}, nil
	}
	if tag := bestUnionTag(in, out); tag != -1 {
		return step{op: castToUnion, fromType: in, toTag: tag, toType: out}, nil
	}
	return step{}, fmt.Errorf("createStep: incompatible types %s and %s", sup.FormatType(in), sup.FormatType(out))
}

// newRecordStep returns a step that will build a record of type out from a
// record of type in. The two types must be compatible, meaning that
// the input type must be an unordered subset of the input type
// (where 'unordered' means that if the output type has record fields
// [a b] and the input type has fields [b a] that is ok). It is also
// ok for leaf primitive types to be different; if they are a casting
// step is inserted.
func newRecordStep(sctx *super.Context, in *super.TypeRecord, out super.Type) (step, error) {
	var children []step
	for _, outField := range super.TypeRecordOf(out).Fields {
		ind, ok := in.IndexOfField(outField.Name)
		if !ok {
			children = append(children, step{op: null, toType: outField.Type})
			continue
		}
		child, err := newStep(sctx, in.Fields[ind].Type, outField.Type)
		if err != nil {
			return step{}, err
		}
		child.fromIndex = ind
		children = append(children, child)
	}
	return step{op: record, toType: out, children: children}, nil
}

func newArrayOrSetStep(sctx *super.Context, op op, inInner, out super.Type) (step, error) {
	innerStep, err := newStep(sctx, inInner, super.InnerType(out))
	if err != nil {
		return step{}, err
	}
	return step{op: op, toType: out, children: []step{innerStep}}, nil
}

// build applies the operation described by s to in, appends the resulting bytes
// to b, and returns the resulting type.  The type is usually s.toType but can
// differ if a primitive cast fails.
func (s *step) build(sctx *super.Context, in zcode.Bytes, b *zcode.Builder) super.Type {
	if in == nil || s.op == copyOp {
		b.Append(in)
		return s.toType
	}
	switch s.op {
	case castPrimitive:
		// For a successful cast, v.Type == super.TypeUnder(s.toType).
		// For a failed cast, v.Type is a super.TypeError.
		v := s.caster.Eval(super.NewValue(s.fromType, in))
		b.Append(v.Bytes())
		if super.TypeUnder(v.Type()) == super.TypeUnder(s.toType) {
			// Prefer s.toType in case it's a named type.
			return s.toType
		}
		return v.Type()
	case castFromUnion:
		it := in.Iter()
		tag := int(super.DecodeInt(it.Next()))
		return s.children[tag].build(sctx, it.Next(), b)
	case castToUnion:
		super.BuildUnion(b, s.toTag, in)
		return s.toType
	case array, set:
		return s.buildArrayOrSet(sctx, s.op, in, b)
	case record:
		return s.buildRecord(sctx, in, b)
	default:
		panic(fmt.Sprintf("unknown step.op %v", s.op))
	}
}

func (s *step) buildArrayOrSet(sctx *super.Context, op op, in zcode.Bytes, b *zcode.Builder) super.Type {
	b.BeginContainer()
	defer b.EndContainer()
	s.types = s.types[:0]
	for it := in.Iter(); !it.Done(); {
		typ := s.children[0].build(sctx, it.Next(), b)
		s.types = append(s.types, typ)
	}
	s.uniqueTypes = append(s.uniqueTypes[:0], s.types...)
	s.uniqueTypes = super.UniqueTypes(s.uniqueTypes)
	var inner super.Type
	switch len(s.uniqueTypes) {
	case 0:
		return s.toType
	case 1:
		inner = s.uniqueTypes[0]
	default:
		union := sctx.LookupTypeUnion(s.uniqueTypes)
		// Convert each container element to the union type.
		b.TransformContainer(func(bytes zcode.Bytes) zcode.Bytes {
			var b2 zcode.Builder
			for i, it := 0, bytes.Iter(); !it.Done(); i++ {
				super.BuildUnion(&b2, union.TagOf(s.types[i]), it.Next())
			}
			return b2.Bytes()
		})
		inner = union
	}
	if op == set {
		b.TransformContainer(super.NormalizeSet)
	}
	if super.TypeUnder(inner) == super.TypeUnder(super.InnerType(s.toType)) {
		// Prefer s.toType in case it or its inner type is a named type.
		return s.toType
	}
	if op == set {
		return sctx.LookupTypeSet(inner)
	}
	return sctx.LookupTypeArray(inner)
}

func (s *step) buildRecord(sctx *super.Context, in zcode.Bytes, b *zcode.Builder) super.Type {
	b.BeginContainer()
	defer b.EndContainer()
	s.types = s.types[:0]
	var needNewRecordType bool
	for _, child := range s.children {
		if child.op == null {
			b.Append(nil)
			s.types = append(s.types, child.toType)
			continue
		}
		// Using getNthFromContainer means we iterate from the
		// beginning of the record for each field. An
		// optimization (for shapes that don't require field
		// reordering) would be make direct use of a
		// zcode.Iter along with keeping track of our
		// position.
		bytes, _ := getNthFromContainer(in, child.fromIndex)
		typ := child.build(sctx, bytes, b)
		if super.TypeUnder(typ) == super.TypeUnder(child.toType) {
			// Prefer child.toType in case it's a named type.
			typ = child.toType
		} else {
			// This field's type differs from the corresponding
			// field in s.toType, so we'll need to look up a new
			// record type below.
			needNewRecordType = true
		}
		s.types = append(s.types, typ)
	}
	if needNewRecordType {
		fields := slices.Clone(super.TypeUnder(s.toType).(*super.TypeRecord).Fields)
		for i, t := range s.types {
			fields[i].Type = t
		}
		return sctx.MustLookupTypeRecord(fields)
	}
	return s.toType
}
