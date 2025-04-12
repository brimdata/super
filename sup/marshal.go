package sup

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/zcode"
	"github.com/x448/float16"
)

//XXX handle new TypeError => marshal as a SUP string?

func Marshal(v any) (string, error) {
	return NewMarshaler().Marshal(v)
}

type MarshalContext struct {
	*MarshalBSUPContext
	formatter *Formatter
}

func NewMarshaler() *MarshalContext {
	return NewMarshalerIndent(0)
}

func NewMarshalerIndent(indent int) *MarshalContext {
	return &MarshalContext{
		MarshalBSUPContext: NewBSUPMarshaler(),
		formatter:          NewFormatter(indent, false, nil),
	}
}

func NewMarshalerWithContext(sctx *super.Context) *MarshalContext {
	return &MarshalContext{
		MarshalBSUPContext: NewBSUPMarshalerWithContext(sctx),
	}
}

func (m *MarshalContext) Marshal(v any) (string, error) {
	val, err := m.MarshalBSUPContext.Marshal(v)
	if err != nil {
		return "", err
	}
	return m.formatter.Format(val), nil
}

func (m *MarshalContext) MarshalCustom(names []string, fields []any) (string, error) {
	rec, err := m.MarshalBSUPContext.MarshalCustom(names, fields)
	if err != nil {
		return "", err
	}
	return m.formatter.FormatRecord(rec), nil
}

type UnmarshalContext struct {
	*UnmarshalBSUPContext
	sctx     *super.Context
	analyzer Analyzer
	builder  *zcode.Builder
}

func NewUnmarshaler() *UnmarshalContext {
	return &UnmarshalContext{
		UnmarshalBSUPContext: NewBSUPUnmarshaler(),
		sctx:                 super.NewContext(),
		analyzer:             NewAnalyzer(),
		builder:              zcode.NewBuilder(),
	}
}

func Unmarshal(sup string, v any) error {
	return NewUnmarshaler().Unmarshal(sup, v)
}

func (u *UnmarshalContext) Unmarshal(sup string, v any) error {
	parser := NewParser(strings.NewReader(sup))
	ast, err := parser.ParseValue()
	if err != nil {
		return err
	}
	val, err := u.analyzer.ConvertValue(u.sctx, ast)
	if err != nil {
		return err
	}
	zedVal, err := Build(u.builder, val)
	if err != nil {
		return nil
	}
	return u.UnmarshalBSUPContext.Unmarshal(zedVal, v)
}

type BSUPMarshaler interface {
	MarshalBSUP(*MarshalBSUPContext) (super.Type, error)
}

func MarshalBSUP(v any) (super.Value, error) {
	return NewBSUPMarshaler().Marshal(v)
}

type MarshalBSUPContext struct {
	*super.Context
	zcode.Builder
	decorator func(string, string) string
	bindings  map[string]string
}

func NewBSUPMarshaler() *MarshalBSUPContext {
	return NewBSUPMarshalerWithContext(super.NewContext())
}

func NewBSUPMarshalerWithContext(sctx *super.Context) *MarshalBSUPContext {
	return &MarshalBSUPContext{
		Context: sctx,
	}
}

// MarshalValue marshals v into the value that is being built and is
// typically called by a custom marshaler.
func (m *MarshalBSUPContext) MarshalValue(v any) (super.Type, error) {
	return m.encodeValue(reflect.ValueOf(v))
}

func (m *MarshalBSUPContext) Marshal(v any) (super.Value, error) {
	m.Builder.Reset()
	typ, err := m.encodeValue(reflect.ValueOf(v))
	if err != nil {
		return super.Null, err
	}
	bytes := m.Builder.Bytes()
	it := bytes.Iter()
	if it.Done() {
		return super.Null, errors.New("no value found")
	}
	return super.NewValue(typ, it.Next()), nil
}

func (m *MarshalBSUPContext) MarshalCustom(names []string, vals []any) (super.Value, error) {
	if len(names) != len(vals) {
		return super.Null, errors.New("names and vals have different lengths")
	}
	m.Builder.Reset()
	var fields []super.Field
	for k, v := range vals {
		typ, err := m.encodeValue(reflect.ValueOf(v))
		if err != nil {
			return super.Null, err
		}
		fields = append(fields, super.Field{Name: names[k], Type: typ})
	}
	// XXX issue #1836
	// Since this can be the inner loop here and nowhere else do we call
	// LookupTypeRecord on the inner loop, now may be the time to put an
	// efficient cache ahead of formatting the fields into a string,
	// e.g., compute a has in place across the field names then do a
	// closed-address exact match for the values in the slot.
	recType, err := m.Context.LookupTypeRecord(fields)
	if err != nil {
		return super.Null, err
	}
	return super.NewValue(recType, m.Builder.Bytes()), nil
}

const (
	tagName = "super"
	tagSep  = ","
)

func fieldName(f reflect.StructField) string {
	tag := f.Tag.Get(tagName)
	if tag == "" {
		tag = f.Tag.Get("json")
	}
	if tag != "" {
		s := strings.SplitN(tag, tagSep, 2)
		if len(s) > 0 && s[0] != "" {
			return s[0]
		}
	}
	return f.Name
}

func typeSimple(name, path string) string {
	return name
}

func typePackage(name, path string) string {
	a := strings.Split(path, "/")
	return fmt.Sprintf("%s.%s", a[len(a)-1], name)
}

func typeFull(name, path string) string {
	return fmt.Sprintf("%s.%s", path, name)
}

type TypeStyle int

const (
	StyleNone TypeStyle = iota
	StyleSimple
	StylePackage
	StyleFull
)

// Decorate informs the marshaler to add type decorations to the resulting BSUP
// in the form of named types in the sytle indicated, e.g.,
// for a `struct Foo` in `package bar` at import path `github.com/acme/bar:
// the corresponding name would be `Foo` for TypeSimple, `bar.Foo` for TypePackage,
// and `github.com/acme/bar.Foo`for TypeFull.  This mechanism works in conjunction
// with Bindings.  Typically you would want just one or the other, but if a binding
// doesn't exist for a given Go type, then a SUP type name will be created according
// to the decorator setting (which may be TypeNone).
func (m *MarshalBSUPContext) Decorate(style TypeStyle) {
	switch style {
	default:
		m.decorator = nil
	case StyleSimple:
		m.decorator = typeSimple
	case StylePackage:
		m.decorator = typePackage
	case StyleFull:
		m.decorator = typeFull
	}
}

// NamedBindings informs the Marshaler to encode the given types with the
// corresponding SUP type names.  For example, to serialize a `bar.Foo`
// value decoroated with the SUP type name "SpecialFoo", simply call
// NamedBindings with the value []Binding{{"SpecialFoo", &bar.Foo{}}.
// Subsequent calls to NamedBindings
// add additional such bindings leaving the existing bindings in place.
// During marshaling, if no binding is found for a particular Go value,
// then the marshaler's decorator setting applies.
func (m *MarshalBSUPContext) NamedBindings(bindings []Binding) error {
	if m.bindings == nil {
		m.bindings = make(map[string]string)
	}
	for _, b := range bindings {
		name, err := typeNameOfValue(b.Template)
		if err != nil {
			return err
		}
		m.bindings[name] = b.Name
	}
	return nil
}

var nanoTsType = reflect.TypeOf(nano.Ts(0))
var superValueType = reflect.TypeOf(super.Value{})

func (m *MarshalBSUPContext) encodeValue(v reflect.Value) (super.Type, error) {
	typ, err := m.encodeAny(v)
	if err != nil {
		return nil, err
	}
	if _, ok := typ.(*super.TypeNamed); ok {
		// We already have a named type.
		return typ, nil
	}
	if !v.IsValid() {
		// v.Type will panic.
		return typ, nil
	}
	return m.lookupTypeNamed(v.Type(), typ)
}

func (m *MarshalBSUPContext) encodeAny(v reflect.Value) (super.Type, error) {
	if !v.IsValid() {
		m.Builder.Append(nil)
		return super.TypeNull, nil
	}
	switch v := v.Interface().(type) {
	case BSUPMarshaler:
		return v.MarshalBSUP(m)
	case float16.Float16:
		m.Builder.Append(super.EncodeFloat16(v.Float32()))
		return super.TypeFloat16, nil
	case nano.Ts:
		m.Builder.Append(super.EncodeTime(v))
		return super.TypeTime, nil
	case net.IP:
		if a, err := netip.ParseAddr(v.String()); err == nil {
			m.Builder.Append(super.EncodeIP(a))
			return super.TypeIP, nil
		}
	case time.Time:
		m.Builder.Append(super.EncodeTime(nano.TimeToTs(v)))
		return super.TypeTime, nil
	case super.Type:
		val := m.Context.LookupTypeValue(v)
		m.Builder.Append(val.Bytes())
		return val.Type(), nil
	case super.Value:
		typ, err := m.TranslateType(v.Type())
		if err != nil {
			return nil, err
		}
		m.Builder.Append(v.Bytes())
		return typ, nil
	}
	switch v.Kind() {
	case reflect.Array:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return m.encodeArrayBytes(v)
		}
		return m.encodeArray(v)
	case reflect.Map:
		if v.IsNil() {
			return m.encodeNil(v.Type())
		}
		return m.encodeMap(v)
	case reflect.Slice:
		if v.IsNil() {
			return m.encodeNil(v.Type())
		}
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return m.encodeSliceBytes(v)
		}
		return m.encodeArray(v)
	case reflect.Struct:
		if a, ok := v.Interface().(netip.Addr); ok {
			m.Builder.Append(super.EncodeIP(a))
			return super.TypeIP, nil
		}
		return m.encodeRecord(v)
	case reflect.Ptr:
		if v.IsNil() {
			return m.encodeNil(v.Type())
		}
		return m.encodeValue(v.Elem())
	case reflect.Interface:
		if v.IsNil() {
			return m.encodeNil(v.Type())
		}
		return m.encodeValue(v.Elem())
	case reflect.String:
		m.Builder.Append(super.EncodeString(v.String()))
		return super.TypeString, nil
	case reflect.Bool:
		m.Builder.Append(super.EncodeBool(v.Bool()))
		return super.TypeBool, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		zt, err := m.lookupType(v.Type())
		if err != nil {
			return nil, err
		}
		m.Builder.Append(super.EncodeInt(v.Int()))
		return zt, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		zt, err := m.lookupType(v.Type())
		if err != nil {
			return nil, err
		}
		m.Builder.Append(super.EncodeUint(v.Uint()))
		return zt, nil
	case reflect.Float32:
		m.Builder.Append(super.EncodeFloat32(float32(v.Float())))
		return super.TypeFloat32, nil
	case reflect.Float64:
		m.Builder.Append(super.EncodeFloat64(v.Float()))
		return super.TypeFloat64, nil
	default:
		return nil, fmt.Errorf("unsupported type: %v", v.Kind())
	}
}

func (m *MarshalBSUPContext) encodeMap(v reflect.Value) (super.Type, error) {
	var lastKeyType, lastValType super.Type
	m.Builder.BeginContainer()
	for it := v.MapRange(); it.Next(); {
		keyType, err := m.encodeValue(it.Key())
		if err != nil {
			return nil, err
		}
		if keyType != lastKeyType && lastKeyType != nil {
			return nil, errors.New("map has mixed key types")
		}
		lastKeyType = keyType
		valType, err := m.encodeValue(it.Value())
		if err != nil {
			return nil, err
		}
		if valType != lastValType && lastValType != nil {
			return nil, errors.New("map has mixed values types")
		}
		lastValType = valType
	}
	m.Builder.TransformContainer(super.NormalizeMap)
	m.Builder.EndContainer()
	if lastKeyType == nil {
		// Map is empty so look up types.
		var err error
		lastKeyType, err = m.lookupType(v.Type().Key())
		if err != nil {
			return nil, err
		}
		lastValType, err = m.lookupType(v.Type().Elem())
		if err != nil {
			return nil, err
		}
	}
	return m.Context.LookupTypeMap(lastKeyType, lastValType), nil
}

func (m *MarshalBSUPContext) encodeNil(t reflect.Type) (super.Type, error) {
	typ, err := m.lookupType(t)
	if err != nil {
		return nil, err
	}
	m.Builder.Append(nil)
	return typ, nil
}

func (m *MarshalBSUPContext) encodeRecord(sval reflect.Value) (super.Type, error) {
	m.Builder.BeginContainer()
	var fields []super.Field
	stype := sval.Type()
	for i := 0; i < stype.NumField(); i++ {
		sf := stype.Field(i)
		isUnexported := sf.PkgPath != ""
		if sf.Anonymous {
			t := sf.Type
			if t.Kind() == reflect.Ptr {
				t = t.Elem()
			}
			if isUnexported && t.Kind() != reflect.Struct {
				// Ignore embedded fields of unexported non-struct types.
				continue
			}
			// Do not ignore embedded fields of unexported struct types
			// since they may have exported fields.
		} else if isUnexported {
			// Ignore unexported non-embedded fields.
			continue
		}
		field := stype.Field(i)
		name := fieldName(field)
		if name == "-" {
			// Ignore fields named "-".
			continue
		}
		typ, err := m.encodeValue(sval.Field(i))
		if err != nil {
			return nil, err
		}
		fields = append(fields, super.Field{Name: name, Type: typ})
	}
	m.Builder.EndContainer()
	return m.Context.LookupTypeRecord(fields)
}

func (m *MarshalBSUPContext) encodeSliceBytes(sliceVal reflect.Value) (super.Type, error) {
	m.Builder.Append(sliceVal.Bytes())
	return super.TypeBytes, nil
}

func (m *MarshalBSUPContext) encodeArrayBytes(arrayVal reflect.Value) (super.Type, error) {
	n := arrayVal.Len()
	bytes := make([]byte, 0, n)
	for k := 0; k < n; k++ {
		v := arrayVal.Index(k)
		bytes = append(bytes, v.Interface().(uint8))
	}
	m.Builder.Append(bytes)
	return super.TypeBytes, nil
}

func (m *MarshalBSUPContext) encodeArray(arrayVal reflect.Value) (super.Type, error) {
	m.Builder.BeginContainer()
	arrayLen := arrayVal.Len()
	types := make([]super.Type, 0, arrayLen)
	for i := 0; i < arrayLen; i++ {
		item := arrayVal.Index(i)
		typ, err := m.encodeValue(item)
		if err != nil {
			return nil, err
		}
		types = append(types, typ)
	}
	uniqueTypes := super.UniqueTypes(slices.Clone(types))
	var innerType super.Type
	switch len(uniqueTypes) {
	case 0:
		// if slice was empty, look up the type without a value
		var err error
		innerType, err = m.lookupType(arrayVal.Type().Elem())
		if err != nil {
			return nil, err
		}
	case 1:
		innerType = types[0]
	default:
		unionType := m.Context.LookupTypeUnion(uniqueTypes)
		// Convert each container element to the union type.
		m.Builder.TransformContainer(func(bytes zcode.Bytes) zcode.Bytes {
			var b zcode.Builder
			for i, it := 0, bytes.Iter(); !it.Done(); i++ {
				super.BuildUnion(&b, unionType.TagOf(types[i]), it.Next())
			}
			return b.Bytes()
		})
		innerType = unionType
	}
	m.Builder.EndContainer()
	return m.Context.LookupTypeArray(innerType), nil
}

func (m *MarshalBSUPContext) lookupType(t reflect.Type) (super.Type, error) {
	var typ super.Type
	switch t.Kind() {
	case reflect.Array, reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			typ = super.TypeBytes
		} else {
			inner, err := m.lookupType(t.Elem())
			if err != nil {
				return nil, err
			}
			typ = m.Context.LookupTypeArray(inner)
		}
	case reflect.Map:
		key, err := m.lookupType(t.Key())
		if err != nil {
			return nil, err
		}
		val, err := m.lookupType(t.Elem())
		if err != nil {
			return nil, err
		}
		typ = m.Context.LookupTypeMap(key, val)
	case reflect.Struct:
		var err error
		typ, err = m.lookupTypeRecord(t)
		if err != nil {
			return nil, err
		}
	case reflect.Ptr:
		var err error
		typ, err = m.lookupType(t.Elem())
		if err != nil {
			return nil, err
		}
	case reflect.String:
		typ = super.TypeString
	case reflect.Bool:
		typ = super.TypeBool
	case reflect.Int, reflect.Int64:
		typ = super.TypeInt64
	case reflect.Int32:
		typ = super.TypeInt32
	case reflect.Int16:
		typ = super.TypeInt16
	case reflect.Int8:
		typ = super.TypeInt8
	case reflect.Uint, reflect.Uint64:
		typ = super.TypeUint64
	case reflect.Uint32:
		typ = super.TypeUint32
	case reflect.Uint16:
		typ = super.TypeUint16
	case reflect.Uint8:
		typ = super.TypeUint8
	case reflect.Float32:
		typ = super.TypeFloat32
	case reflect.Float64:
		typ = super.TypeFloat64
	case reflect.Interface:
		// Encode interfaces when we don't know the underlying concrete type as null type.
		typ = super.TypeNull
	default:
		return nil, fmt.Errorf("unsupported type: %v", t.Kind())
	}
	return m.lookupTypeNamed(t, typ)
}

func (m *MarshalBSUPContext) lookupTypeRecord(structType reflect.Type) (super.Type, error) {
	var fields []super.Field
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		name := fieldName(field)
		fieldType, err := m.lookupType(field.Type)
		if err != nil {
			return nil, err
		}
		fields = append(fields, super.Field{Name: name, Type: fieldType})
	}
	return m.Context.LookupTypeRecord(fields)
}

// lookupTypeNamed returns a named type for typ with a name derived from t.  It
// returns typ if it shouldn't derive a name from t.
func (m *MarshalBSUPContext) lookupTypeNamed(t reflect.Type, typ super.Type) (super.Type, error) {
	if m.decorator == nil && m.bindings == nil {
		return typ, nil
	}
	// Don't create named types for interface types as this is just
	// one value for that interface and it's the underlying concrete
	// types that implement the interface that we want to name.
	if t.Kind() == reflect.Interface {
		return typ, nil
	}
	// We do not want to further decorate nano.Ts as
	// it's already been converted to a Zed time;
	// likewise for super.Value, which gets encoded as
	// itself and its own named type if it has one.
	if t == nanoTsType || t == superValueType || t == netipAddrType || t == netIPType {
		return typ, nil
	}
	name := t.Name()
	if name == "" || name == t.Kind().String() {
		return typ, nil
	}
	path := t.PkgPath()
	var named string
	if m.bindings != nil {
		named = m.bindings[typeFull(name, path)]
	}
	if named == "" && m.decorator != nil {
		named = m.decorator(name, path)
	}
	if named == "" {
		return typ, nil
	}
	return m.Context.LookupTypeNamed(named, typ)
}

type BSUPUnmarshaler interface {
	UnmarshalBSUP(*UnmarshalBSUPContext, super.Value) error
}

type UnmarshalBSUPContext struct {
	sctx   *super.Context
	binder binder
}

func NewBSUPUnmarshaler() *UnmarshalBSUPContext {
	return &UnmarshalBSUPContext{}
}

func UnmarshalBSUP(val super.Value, v any) error {
	return NewBSUPUnmarshaler().decodeAny(val, reflect.ValueOf(v))
}

func incompatTypeError(zt super.Type, v reflect.Value) error {
	return fmt.Errorf("incompatible type translation: Super type %v, Go type %v, Go kind %v", FormatType(zt), v.Type(), v.Kind())
}

// SetContext provides an optional type context to the unmarshaler.  This is
// needed only when unmarshaling Zed type values into Go super.Type interface values.
func (u *UnmarshalBSUPContext) SetContext(sctx *super.Context) {
	u.sctx = sctx
}

func (u *UnmarshalBSUPContext) Unmarshal(val super.Value, v any) error {
	return u.decodeAny(val, reflect.ValueOf(v))
}

// Bindings informs the unmarshaler that SUP values with a type name equal
// to any of the three variations of Go type mame (full path, package.Type,
// or just Type) may be used to inform the deserialization of a SUP value
// into a Go interface value.  If full path names are not used, it is up to
// the entitity that marshaled the original SUP to ensure that no type-name
// conflicts arise, e.g., when using the TypeSimple decorator style, you cannot
// have a type called bar.Foo and another type baz.Foo as the simple type
// decorator will be "Foo" in both cases and thus create a name conflict.
func (u *UnmarshalBSUPContext) Bind(templates ...any) error {
	for _, t := range templates {
		if err := u.binder.enterTemplate(t); err != nil {
			return err
		}
	}
	return nil
}

func (u *UnmarshalBSUPContext) NamedBindings(bindings []Binding) error {
	for _, b := range bindings {
		if err := u.binder.enterBinding(b); err != nil {
			return err
		}
	}
	return nil
}

var netipAddrType = reflect.TypeOf(netip.Addr{})
var netIPType = reflect.TypeOf(net.IP{})

func (u *UnmarshalBSUPContext) decodeAny(val super.Value, v reflect.Value) (x error) {
	if !v.IsValid() {
		return errors.New("cannot unmarshal into value provided")
	}
	m, v := indirect(v, val)
	if m != nil {
		return m.UnmarshalBSUP(u, val)
	}
	switch v.Interface().(type) {
	case float16.Float16:
		if val.Type() != super.TypeFloat16 {
			return incompatTypeError(val.Type(), v)
		}
		v.SetUint(uint64(float16.Fromfloat32(float32(val.Float()))))
		return nil
	case nano.Ts:
		if val.Type() != super.TypeTime {
			return incompatTypeError(val.Type(), v)
		}
		v.Set(reflect.ValueOf(super.DecodeTime(val.Bytes())))
		return nil
	case super.Value:
		// For super.Values we simply set the reflect value to the
		// super.Value that has been decoded.
		v.Set(reflect.ValueOf(val.Copy()))
		return nil
	}
	if super.TypeUnder(val.Type()) == super.TypeNull {
		// A zed null value should successfully unmarshal to any go type. Typed
		// nulls however need to be type checked.
		v.Set(reflect.Zero(v.Type()))
		return nil
	}
	if v.Kind() == reflect.Pointer && val.IsNull() {
		return u.decodeNull(val, v)
	}
	switch v.Kind() {
	case reflect.Array:
		return u.decodeArray(val, v)
	case reflect.Map:
		return u.decodeMap(val, v)
	case reflect.Slice:
		if v.Type() == netIPType {
			return u.decodeNetIP(val, v)
		}
		return u.decodeArray(val, v)
	case reflect.Struct:
		if v.Type() == netipAddrType {
			return u.decodeNetipAddr(val, v)
		}
		return u.decodeRecord(val, v)
	case reflect.Interface:
		if super.TypeUnder(val.Type()) == super.TypeType {
			if u.sctx == nil {
				return errors.New("cannot unmarshal type value without type context")
			}
			typ, err := u.sctx.LookupByValue(val.Bytes())
			if err != nil {
				return err
			}
			v.Set(reflect.ValueOf(typ))
			return nil
		}
		// If the interface value isn't null, then the user has provided
		// an underlying value to unmarshal into.  So we just recursively
		// decode the value into this existing value and return.
		if !v.IsNil() {
			return u.decodeAny(val, v.Elem())
		}
		template, err := u.lookupGoType(val.Type(), val.Bytes())
		if err != nil {
			return err
		}
		if template == nil {
			// If the template is nil, then the value must be of BSUP type null
			// and BSUP type values can only have value null.  So, we
			// set it to null of the type given for the marshaled-into
			// value and return.
			v.Set(reflect.Zero(v.Type()))
			return nil
		}
		concrete := reflect.New(template)
		if err := u.decodeAny(val, concrete.Elem()); err != nil {
			return err
		}
		// For empty interface, we pull the value pointed-at into the
		// empty-interface value if it's not a struct (i.e., a scalar or
		// a slice)  For normal interfaces, we set the pointer to be
		// the pointer to the new object as it must be type-compatible.
		if v.NumMethod() == 0 && concrete.Elem().Kind() != reflect.Struct {
			v.Set(concrete.Elem())
		} else {
			v.Set(concrete)
		}
		return nil
	case reflect.String:
		// XXX We bundle string, type, error all into string.
		// See issue #1853.
		switch super.TypeUnder(val.Type()) {
		case super.TypeString, super.TypeType:
		default:
			return incompatTypeError(val.Type(), v)
		}
		v.SetString(super.DecodeString(val.Bytes()))
		return nil
	case reflect.Bool:
		if super.TypeUnder(val.Type()) != super.TypeBool {
			return incompatTypeError(val.Type(), v)
		}
		v.SetBool(val.Bool())
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch super.TypeUnder(val.Type()) {
		case super.TypeInt8, super.TypeInt16, super.TypeInt32, super.TypeInt64:
		default:
			return incompatTypeError(val.Type(), v)
		}
		v.SetInt(val.Int())
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		switch super.TypeUnder(val.Type()) {
		case super.TypeUint8, super.TypeUint16, super.TypeUint32, super.TypeUint64:
		default:
			return incompatTypeError(val.Type(), v)
		}
		v.SetUint(val.Uint())
		return nil
	case reflect.Float32:
		if super.TypeUnder(val.Type()) != super.TypeFloat32 {
			return incompatTypeError(val.Type(), v)
		}
		v.SetFloat(val.Float())
		return nil
	case reflect.Float64:
		if super.TypeUnder(val.Type()) != super.TypeFloat64 {
			return incompatTypeError(val.Type(), v)
		}
		v.SetFloat(val.Float())
		return nil
	default:
		return fmt.Errorf("unsupported type: %v", v.Kind())
	}
}

// Adapted from:
// https://github.com/golang/go/blob/46ab7a5c4f80d912f25b6b3e1044282a2a79df8b/src/encoding/json/decode.go#L426
func indirect(v reflect.Value, val super.Value) (BSUPUnmarshaler, reflect.Value) {
	// If v is a named type and is addressable,
	// start with its address, so that if the type has pointer methods,
	// we find them.
	if v.Kind() != reflect.Pointer && v.Type().Name() != "" && v.CanAddr() {
		v = v.Addr()
	}
	var nilptr reflect.Value
	for v.Kind() == reflect.Pointer {
		if v.CanSet() && val.IsNull() {
			// If the reflect value can be set and the zed Value is nil we want
			// to store this pointer since if destination is not a super.Value the
			// pointer will be set to nil.
			nilptr = v
		}
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		if v.Type().NumMethod() > 0 && v.CanInterface() {
			if u, ok := v.Interface().(BSUPUnmarshaler); ok {
				return u, reflect.Value{}
			}
		}
		v = v.Elem()
	}
	if _, ok := v.Interface().(super.Value); !ok && nilptr.IsValid() {
		return nil, nilptr
	}
	return nil, v
}

func (u *UnmarshalBSUPContext) decodeNull(val super.Value, v reflect.Value) error {
	inner := v
	for inner.Kind() == reflect.Ptr {
		if inner.IsNil() {
			// Set nil elements so we can find the actual type of the underlying
			// value. This is not so we can set the type since the outer value
			// will eventually get set to nil- but rather so we can type check
			// the null (i.e., you cannot marshal a int64 to null(ip), etc.).
			v.Set(reflect.New(v.Type().Elem()))
		}
		inner = inner.Elem()
	}
	if err := u.decodeAny(val, inner); err != nil {
		return err
	}
	v.Set(reflect.Zero(v.Type()))
	return nil
}

func (u *UnmarshalBSUPContext) decodeNetipAddr(val super.Value, v reflect.Value) error {
	if super.TypeUnder(val.Type()) != super.TypeIP {
		return incompatTypeError(val.Type(), v)
	}
	v.Set(reflect.ValueOf(super.DecodeIP(val.Bytes())))
	return nil
}

func (u *UnmarshalBSUPContext) decodeNetIP(val super.Value, v reflect.Value) error {
	if super.TypeUnder(val.Type()) != super.TypeIP {
		return incompatTypeError(val.Type(), v)
	}
	v.Set(reflect.ValueOf(net.ParseIP(super.DecodeIP(val.Bytes()).String())))
	return nil
}

func (u *UnmarshalBSUPContext) decodeMap(val super.Value, mapVal reflect.Value) error {
	typ, ok := super.TypeUnder(val.Type()).(*super.TypeMap)
	if !ok {
		return errors.New("not a map")
	}
	if val.IsNull() {
		// XXX The inner types of the null should be checked.
		mapVal.Set(reflect.Zero(mapVal.Type()))
		return nil
	}
	if mapVal.IsNil() {
		mapVal.Set(reflect.MakeMap(mapVal.Type()))
	}
	keyType := mapVal.Type().Key()
	valType := mapVal.Type().Elem()
	for it := val.Iter(); !it.Done(); {
		key := reflect.New(keyType).Elem()
		if err := u.decodeAny(super.NewValue(typ.KeyType, it.Next()), key); err != nil {
			return err
		}
		val := reflect.New(valType).Elem()
		if err := u.decodeAny(super.NewValue(typ.ValType, it.Next()), val); err != nil {
			return err
		}
		mapVal.SetMapIndex(key, val)
	}
	return nil
}

func (u *UnmarshalBSUPContext) decodeRecord(val super.Value, sval reflect.Value) error {
	if union, ok := val.Type().(*super.TypeUnion); ok {
		typ, bytes := union.Untag(val.Bytes())
		val = super.NewValue(typ, bytes)
	}
	recType, ok := super.TypeUnder(val.Type()).(*super.TypeRecord)
	if !ok {
		return fmt.Errorf("cannot unmarshal Zed value %q into Go struct", String(val))
	}
	nameToField := make(map[string]int)
	stype := sval.Type()
	for i := 0; i < stype.NumField(); i++ {
		field := stype.Field(i)
		name := fieldName(field)
		nameToField[name] = i
	}
	for i, it := 0, val.Iter(); !it.Done(); i++ {
		if i >= len(recType.Fields) {
			return errors.New("malformed Zed value")
		}
		itzv := it.Next()
		name := recType.Fields[i].Name
		if fieldIdx, ok := nameToField[name]; ok {
			typ := recType.Fields[i].Type
			if err := u.decodeAny(super.NewValue(typ, itzv), sval.Field(fieldIdx)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (u *UnmarshalBSUPContext) decodeArray(val super.Value, arrVal reflect.Value) error {
	typ := super.TypeUnder(val.Type())
	if typ == super.TypeBytes && arrVal.Type().Elem().Kind() == reflect.Uint8 {
		if val.IsNull() {
			arrVal.Set(reflect.Zero(arrVal.Type()))
			return nil
		}
		if arrVal.Kind() == reflect.Array {
			return u.decodeArrayBytes(val, arrVal)
		}
		// arrVal is a slice here.
		arrVal.SetBytes(val.Bytes())
		return nil
	}
	arrType, ok := typ.(*super.TypeArray)
	if !ok {
		return fmt.Errorf("unmarshaling type %q: not an array", String(typ))
	}
	if val.IsNull() {
		// XXX The inner type of the null should be checked.
		arrVal.Set(reflect.Zero(arrVal.Type()))
		return nil
	}
	i := 0
	for it := val.Iter(); !it.Done(); i++ {
		itzv := it.Next()
		if i >= arrVal.Cap() {
			newcap := arrVal.Cap() + arrVal.Cap()/2
			if newcap < 4 {
				newcap = 4
			}
			newArr := reflect.MakeSlice(arrVal.Type(), arrVal.Len(), newcap)
			reflect.Copy(newArr, arrVal)
			arrVal.Set(newArr)
		}
		if i >= arrVal.Len() {
			arrVal.SetLen(i + 1)
		}
		if err := u.decodeAny(super.NewValue(arrType.Type, itzv), arrVal.Index(i)); err != nil {
			return err
		}
	}
	switch {
	case i == 0:
		arrVal.Set(reflect.MakeSlice(arrVal.Type(), 0, 0))
	case i < arrVal.Len():
		arrVal.SetLen(i)
	}
	return nil
}

func (u *UnmarshalBSUPContext) decodeArrayBytes(val super.Value, arrayVal reflect.Value) error {
	if len(val.Bytes()) != arrayVal.Len() {
		return errors.New("BSUP bytes value length differs from Go array")
	}
	for k, b := range val.Bytes() {
		arrayVal.Index(k).Set(reflect.ValueOf(b))
	}
	return nil
}

type Binding struct {
	Name     string // user-defined name
	Template any    // zero-valued entity used as template for new such objects
}

type binding struct {
	key      string
	template reflect.Type
}

type binder map[string][]binding

func (b binder) lookup(name string) reflect.Type {
	if b == nil {
		return nil
	}
	for _, binding := range b[name] {
		if binding.key == name {
			return binding.template
		}
	}
	return nil
}

func (b *binder) enter(key string, typ reflect.Type) error {
	if *b == nil {
		*b = make(map[string][]binding)
	}
	slot := (*b)[key]
	entry := binding{
		key:      key,
		template: typ,
	}
	(*b)[key] = append(slot, entry)
	return nil
}

func (b *binder) enterTemplate(template any) error {
	typ, err := typeOfTemplate(template)
	if err != nil {
		return err
	}
	pkgPath := typ.PkgPath()
	path := strings.Split(pkgPath, "/")
	pkgName := path[len(path)-1]

	// e.g., Foo
	typeName := typ.Name()
	// e.g., bar.Foo
	dottedName := fmt.Sprintf("%s.%s", pkgName, typeName)
	// e.g., github.com/acme/pkg/bar.Foo
	fullName := fmt.Sprintf("%s.%s", pkgPath, typeName)

	if err := b.enter(typeName, typ); err != nil {
		return err
	}
	if err := b.enter(dottedName, typ); err != nil {
		return err
	}
	return b.enter(fullName, typ)
}

func (b *binder) enterBinding(binding Binding) error {
	typ, err := typeOfTemplate(binding.Template)
	if err != nil {
		return err
	}
	return b.enter(binding.Name, typ)
}

func typeOfTemplate(template any) (reflect.Type, error) {
	v := reflect.ValueOf(template)
	if !v.IsValid() {
		return nil, errors.New("invalid template")
	}
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	return v.Type(), nil
}

func typeNameOfValue(value any) (string, error) {
	typ, err := typeOfTemplate(value)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s", typ.PkgPath(), typ.Name()), nil
}

// lookupGoType builds a Go type for the Zed value given by typ and bytes.
// This process requires
// a value rather than a Zed type as it must determine the types of union elements
// from their tags.
func (u *UnmarshalBSUPContext) lookupGoType(typ super.Type, bytes zcode.Bytes) (reflect.Type, error) {
	switch typ := typ.(type) {
	case *super.TypeNamed:
		if template := u.binder.lookup(typ.Name); template != nil {
			return template, nil
		}
		// Ignore named types for which there are no bindings.
		// If an interface type being marshaled into doesn't
		// have a binding, then a type mismatch will be caught
		// by reflect when the Set() method is called on the
		// value and the concrete value doesn't implement the
		// interface.
		return u.lookupGoType(typ.Type, bytes)
	case *super.TypeRecord:
		return nil, errors.New("unmarshaling records into interface value requires type binding")
	case *super.TypeArray:
		// If we got here, we know the array type wasn't named and
		// therefore cannot have mixed-type elements.  So we don't need
		// to traverse the array and can just take the first element
		// as the template value to recurse upon.  If there are actually
		// heterogenous values, then the Go reflect package will raise
		// the problem when decoding the value.
		// If the inner type is a union, it must be a named-type union
		// so we know what Go type to use as the elements of the array,
		// which obviously can only be interface values for mixed types.
		// XXX there's a corner case here for union type where all the
		// elements of the array have the same tag, in which case you
		// can have a normal array of that tag's type.
		// We let the reflect package catch errors where the array contents
		// are not consistent.  All we need to do here is make sure the
		// interface name is in the bindings and the elemType will be
		// the appropriate interface type.
		it := bytes.Iter()
		if it.Done() {
			bytes = nil
		} else {
			bytes = it.Next()
		}
		elemType, err := u.lookupGoType(typ.Type, bytes)
		if err != nil {
			return nil, err
		}
		return reflect.SliceOf(elemType), nil
	case *super.TypeSet:
		// See comment above for TypeArray as it applies here.
		it := bytes.Iter()
		if it.Done() {
			bytes = nil
		} else {
			bytes = it.Next()
		}
		elemType, err := u.lookupGoType(typ.Type, bytes)
		if err != nil {
			return nil, err
		}
		return reflect.SliceOf(elemType), nil
	case *super.TypeUnion:
		return u.lookupGoType(typ.Untag(bytes))
	case *super.TypeEnum:
		// For now just return nil here. The layer above will flag
		// a type error.  At some point, we can create Go-native data structures
		// in package super for representing a union or enum as a standalone
		// entity.  See issue #1853.
		return nil, nil
	case *super.TypeMap:
		it := bytes.Iter()
		if it.Done() {
			return nil, fmt.Errorf("corrupt Zed map value in Zed unmarshal: type %q", String(typ))
		}
		keyType, err := u.lookupGoType(typ.KeyType, it.Next())
		if err != nil {
			return nil, err
		}
		if it.Done() {
			return nil, fmt.Errorf("corrupt Zed map value in Zed unmarshal: type %q", String(typ))
		}
		valType, err := u.lookupGoType(typ.ValType, it.Next())
		if err != nil {
			return nil, err
		}
		return reflect.MapOf(keyType, valType), nil
	default:
		return u.lookupPrimitiveType(typ)
	}
}

func (u *UnmarshalBSUPContext) lookupPrimitiveType(typ super.Type) (reflect.Type, error) {
	var v any
	switch typ := typ.(type) {
	// XXX We should have counterparts for error and type type.
	// See issue #1853.
	// XXX udpate issue?
	case *super.TypeOfString, *super.TypeOfType:
		v = ""
	case *super.TypeOfBool:
		v = false
	case *super.TypeOfUint8:
		v = uint8(0)
	case *super.TypeOfUint16:
		v = uint16(0)
	case *super.TypeOfUint32:
		v = uint32(0)
	case *super.TypeOfUint64:
		v = uint64(0)
	case *super.TypeOfInt8:
		v = int8(0)
	case *super.TypeOfInt16:
		v = int16(0)
	case *super.TypeOfInt32:
		v = int32(0)
	case *super.TypeOfInt64:
		v = int64(0)
	case *super.TypeOfFloat16:
		v = float16.Fromfloat32(0)
	case *super.TypeOfFloat32:
		v = float32(0)
	case *super.TypeOfFloat64:
		v = float64(0)
	case *super.TypeOfIP:
		v = netip.Addr{}
	case *super.TypeOfNet:
		v = net.IPNet{}
	case *super.TypeOfTime:
		v = time.Time{}
	case *super.TypeOfDuration:
		v = time.Duration(0)
	case *super.TypeOfNull:
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown Super type: %v", typ)
	}
	return reflect.TypeOf(v), nil
}
