package resolver

import (
	"errors"
	"fmt"
	"net"
	"reflect"
	"strings"
	"time"

	"github.com/brimsec/zq/pkg/nano"
	"github.com/brimsec/zq/zcode"
	"github.com/brimsec/zq/zng"
)

var (
	errNotStruct = errors.New("not a struct or struct ptr")

	marshalerType   = reflect.TypeOf((*Marshaler)(nil)).Elem()
	unmarshalerType = reflect.TypeOf((*Unmarshaler)(nil)).Elem()
)

type Marshaler interface {
	MarshalZNG(*MarshalContext) (zng.Type, error)
}

func Marshal(v interface{}) (zng.Value, error) {
	return NewMarshaler().Marshal(v)
}

type MarshalContext struct {
	*Context
	zcode.Builder
	decorator func(string, string) string
	bindings  map[string]string
}

func NewMarshaler() *MarshalContext {
	return NewMarshalerWithContext(NewContext())
}

func NewMarshalerWithContext(zctx *Context) *MarshalContext {
	return &MarshalContext{
		Context: zctx,
	}
}

// MarshalValue marshals v into the value that is being built and is
// typically called by a custom marshaler.
func (m *MarshalContext) MarshalValue(v interface{}) (zng.Type, error) {
	return m.encodeValue(reflect.ValueOf(v))
}

func (m *MarshalContext) Marshal(v interface{}) (zng.Value, error) {
	m.Builder.Reset()
	typ, err := m.encodeValue(reflect.ValueOf(v))
	if err != nil {
		return zng.Value{}, err
	}
	bytes := m.Builder.Bytes()
	it := bytes.Iter()
	if it.Done() {
		return zng.Value{}, errors.New("no value found")
	}
	bytes, _, err = it.Next()
	if err != nil {
		return zng.Value{}, err
	}
	return zng.Value{typ, bytes}, nil
}

func (m *MarshalContext) MarshalRecord(v interface{}) (*zng.Record, error) {
	m.Builder.Reset()
	typ, err := m.encodeValue(reflect.ValueOf(v))
	if err != nil {
		return nil, err
	}
	recType, ok := typ.(*zng.TypeRecord)
	if !ok {
		return nil, errors.New("not a record")
	}
	body, err := m.Builder.Bytes().ContainerBody()
	if err != nil {
		return nil, err
	}
	return zng.NewRecord(recType, body), nil
}

func (m *MarshalContext) MarshalCustom(names []string, fields []interface{}) (*zng.Record, error) {
	if len(names) != len(fields) {
		return nil, errors.New("fields and columns don't match")
	}
	m.Builder.Reset()
	var cols []zng.Column
	for k, field := range fields {
		typ, err := m.encodeValue(reflect.ValueOf(field))
		if err != nil {
			return nil, err
		}
		cols = append(cols, zng.Column{names[k], typ})
	}
	// XXX issue #1836
	// Since this can be the inner loop here and nowhere else do we call
	// LookupTypeRecord on the inner loop, now may be the time to put an
	// efficient cache ahead of formatting the columns into a string,
	// e.g., compute a has in place across the field names then do a
	// closed-address exact match for the values in the slot.
	recType, err := m.Context.LookupTypeRecord(cols)
	if err != nil {
		return nil, err
	}
	return zng.NewRecord(recType, m.Builder.Bytes()), nil
}

const (
	tagName = "zng"
	tagSep  = ","
)

func fieldName(f reflect.StructField) string {
	tag := f.Tag.Get(tagName)
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

// Decorate informs the marshaler to add type decorations to the resulting ZNG
// in the form of type aliases that are named in the sytle indicated, e.g.,
// for a `struct Foo` in `package bar` at import path `github.com/acme/bar:
// the corresponding name would be `Foo` for TypeSimple, `bar.Foo` for TypePackage,
// and `github.com/acme/bar.Foo`for TypeFull.  This mechanism works in conjunction
// with Bindings.  Typically you would want just one or the other, but if a binding
// doesn't exist for a given Go type, then a ZSON type name will be created according
// to the decorator setting (which may be TypeNone).
func (m *MarshalContext) Decorate(style TypeStyle) {
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
// corresponding ZSON type names.  For example, to serialize a `bar.Foo`
// value decoroated with the ZSON type name "SpecialFoo", simply call
// NamedBindings with the value []Binding{{"SpecialFoo", &bar.Foo{}}.
// Subsequent calls to NamedBindings
// add additional such bindings leaving the existing bindings in place.
// During marshaling, if no binding is found for a particular Go value,
// then the marshaler's decorator setting applies.
func (m *MarshalContext) NamedBindings(bindings []Binding) error {
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

func (m *MarshalContext) encodeValue(v reflect.Value) (zng.Type, error) {
	typ, err := m.encodeAny(v)
	if err != nil {
		return nil, err
	}
	if m.decorator != nil || m.bindings != nil {
		// Don't create aliases for interface types as this is just
		// one value for that interface and it's the underlying concrete
		// types that implement the interface that we want to alias.
		if !v.IsValid() || v.Kind() == reflect.Interface {
			return typ, nil
		}
		name := v.Type().Name()
		kind := v.Kind().String()
		if name != "" && name != kind {
			path := v.Type().PkgPath()
			var alias string
			if m.bindings != nil {
				alias = m.bindings[typeFull(name, path)]
			}
			if alias == "" && m.decorator != nil {
				alias = m.decorator(name, path)
			}
			if alias != "" {
				return m.Context.LookupTypeAlias(alias, typ)
			}
		}
	}
	return typ, nil
}

func (m *MarshalContext) encodeAny(v reflect.Value) (zng.Type, error) {
	if !v.IsValid() {
		m.Builder.AppendPrimitive(nil)
		return zng.TypeNull, nil
	}
	if v.Type().Implements(marshalerType) {
		return v.Interface().(Marshaler).MarshalZNG(m)
	}
	if v, ok := v.Interface().(nano.Ts); ok {
		m.Builder.AppendPrimitive(zng.EncodeTime(v))
		return zng.TypeTime, nil
	}
	switch v.Kind() {
	case reflect.Array:
		return m.encodeArray(v)
	case reflect.Slice:
		if v.IsNil() {
			return m.encodeNil(v.Type())
		}
		return m.encodeArray(v)
	case reflect.Struct:
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
		m.Builder.AppendPrimitive(zng.EncodeString(v.String()))
		return zng.TypeString, nil
	case reflect.Bool:
		m.Builder.AppendPrimitive(zng.EncodeBool(v.Bool()))
		return zng.TypeBool, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		zt, err := m.lookupType(v.Type())
		if err != nil {
			return nil, err
		}
		m.Builder.AppendPrimitive(zng.EncodeInt(v.Int()))
		return zt, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		zt, err := m.lookupType(v.Type())
		if err != nil {
			return nil, err
		}
		m.Builder.AppendPrimitive(zng.EncodeUint(v.Uint()))
		return zt, nil
	// XXX add float32 to zng?
	case reflect.Float64, reflect.Float32:
		m.Builder.AppendPrimitive(zng.EncodeFloat64(v.Float()))
		return zng.TypeFloat64, nil
	default:
		return nil, fmt.Errorf("unsupported type: %v", v.Kind())
	}
}

func (m *MarshalContext) encodeNil(t reflect.Type) (zng.Type, error) {
	typ, err := m.lookupType(t)
	if err != nil {
		return nil, err
	}
	if zng.IsContainerType(typ) {
		m.Builder.AppendContainer(nil)
	} else {
		m.Builder.AppendPrimitive(nil)
	}
	return typ, nil
}

func (m *MarshalContext) encodeRecord(sval reflect.Value) (zng.Type, error) {
	m.Builder.BeginContainer()
	var columns []zng.Column
	stype := sval.Type()
	for i := 0; i < stype.NumField(); i++ {
		field := stype.Field(i)
		name := fieldName(field)
		typ, err := m.encodeValue(sval.Field(i))
		if err != nil {
			return nil, err
		}
		columns = append(columns, zng.Column{name, typ})
	}
	m.Builder.EndContainer()
	return m.Context.LookupTypeRecord(columns)
}

func isIP(typ reflect.Type) bool {
	return typ.Name() == "IP" && typ.PkgPath() == "net"
}

func (m *MarshalContext) encodeArray(arrayVal reflect.Value) (zng.Type, error) {
	if isIP(arrayVal.Type()) {
		m.Builder.AppendPrimitive(zng.EncodeIP(arrayVal.Bytes()))
		return zng.TypeIP, nil
	}
	len := arrayVal.Len()
	m.Builder.BeginContainer()
	var innerType zng.Type
	for i := 0; i < len; i++ {
		item := arrayVal.Index(i)
		typ, err := m.encodeValue(item)
		if err != nil {
			return nil, err
		}
		innerType = typ
	}
	m.Builder.EndContainer()
	if innerType == nil {
		// if slice was empty, look up the type without a value
		var err error
		innerType, err = m.lookupType(arrayVal.Type().Elem())
		if err != nil {
			return nil, err
		}
	}
	return m.Context.LookupTypeArray(innerType), nil
}

func (m *MarshalContext) lookupType(typ reflect.Type) (zng.Type, error) {
	switch typ.Kind() {
	case reflect.Array, reflect.Slice:
		typ, err := m.lookupType(typ.Elem())
		if err != nil {
			return nil, err
		}
		return m.Context.LookupTypeArray(typ), nil
	case reflect.Struct:
		return m.lookupTypeRecord(typ)
	case reflect.Ptr:
		return m.lookupType(typ.Elem())
	case reflect.String:
		return zng.TypeString, nil
	case reflect.Bool:
		return zng.TypeBool, nil
	case reflect.Int, reflect.Int64:
		return zng.TypeInt64, nil
	case reflect.Int32:
		return zng.TypeInt32, nil
	case reflect.Int16:
		return zng.TypeInt16, nil
	case reflect.Int8:
		return zng.TypeInt8, nil
	case reflect.Uint, reflect.Uint64:
		return zng.TypeUint64, nil
	case reflect.Uint32:
		return zng.TypeUint32, nil
	case reflect.Uint16:
		return zng.TypeUint16, nil
	case reflect.Uint8:
		return zng.TypeUint8, nil
	case reflect.Float64, reflect.Float32:
		return zng.TypeUint64, nil
	default:
		return nil, fmt.Errorf("unsupported type: %v", typ.Kind())
	}
}

func (m *MarshalContext) lookupTypeRecord(structType reflect.Type) (zng.Type, error) {
	var columns []zng.Column
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		name := fieldName(field)
		fieldType, err := m.lookupType(field.Type)
		if err != nil {
			return nil, err
		}
		columns = append(columns, zng.Column{name, fieldType})
	}
	return m.Context.LookupTypeRecord(columns)
}

type Unmarshaler interface {
	UnmarshalZNG(*UnmarshalContext, zng.Value) error
}

type UnmarshalContext struct {
	binder binder
}

func NewUnmarshaler() *UnmarshalContext {
	return &UnmarshalContext{}
}

func Unmarshal(zv zng.Value, v interface{}) error {
	return NewUnmarshaler().decodeAny(zv, reflect.ValueOf(v))
}

func UnmarshalRecord(rec *zng.Record, v interface{}) error {
	return NewUnmarshaler().decodeAny(zng.Value{rec.Alias, rec.Raw}, reflect.ValueOf(v))
}

func incompatTypeError(zt zng.Type, v reflect.Value) error {
	return fmt.Errorf("incompatible type translation: zng type %v go type %v go kind %v", zt, v.Type(), v.Kind())
}

func (u *UnmarshalContext) Unmarshal(zv zng.Value, v interface{}) error {
	return u.decodeAny(zv, reflect.ValueOf(v))
}

// Bindings informs the unmarshaler that ZSON values with a type name equal
// to any of the three variations of Go type mame (full path, package.Type,
// or just Type) may be used to inform the deserialization of a ZSON value
// into a Go interface value.  If full path names are not used, it is up to
// the entitity that marshaled the original ZSON to ensure that no type-name
// conflicts arise, e.g., when using the TypeSimple decorator style, you cannot
// have a type called bar.Foo and another type baz.Foo as the simple type
// decorator will be "Foo" in both cases and thus create a name conflict.
func (u *UnmarshalContext) Bind(templates ...interface{}) error {
	for _, t := range templates {
		if err := u.binder.enterTemplate(t); err != nil {
			return err
		}
	}
	return nil
}

func (u *UnmarshalContext) NamedBindings(bindings []Binding) error {
	for _, b := range bindings {
		if err := u.binder.enterBinding(b); err != nil {
			return err
		}
	}
	return nil
}

func (u *UnmarshalContext) decodeAny(zv zng.Value, v reflect.Value) error {
	if !v.IsValid() {
		return errors.New("cannot unmarshal into value provided")
	}
	if v.Type().Implements(unmarshalerType) {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		return v.Interface().(Unmarshaler).UnmarshalZNG(u, zv)
	}
	if _, ok := v.Interface().(nano.Ts); ok {
		if zv.Type != zng.TypeTime {
			return incompatTypeError(zv.Type, v)
		}
		if zv.Bytes == nil {
			v.Set(reflect.Zero(v.Type()))
			return nil
		}
		x, err := zng.DecodeTime(zv.Bytes)
		v.Set(reflect.ValueOf(x))
		return err
	}
	switch v.Kind() {
	case reflect.Array:
		return u.decodeArray(zv, v)
	case reflect.Slice:
		if isIP(v.Type()) {
			return u.decodeIP(zv, v)
		}
		return u.decodeArray(zv, v)
	case reflect.Struct:
		return u.decodeRecord(zv, v)
	case reflect.Interface:
		// This is an interface value.  If the underlying zng data
		// has a type name (via alias), then we'll see if there's a
		// binding fot it and unmarshal into an instance of Template
		// in the binding.
		typ, err := u.lookupType(zv.Type)
		if err != nil {
			return err
		}
		if typ == nil {
			// If typ is nil, then the value must be of ZNG type null
			// and ZNG type values can only have value null.  So, we
			// set it to null of the type given for the marshaled-into
			// value and return.
			v.Set(reflect.Zero(v.Type()))
			return nil
		}
		concrete := reflect.New(typ)
		if err := u.decodeAny(zv, concrete.Elem()); err != nil {
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
	case reflect.Ptr:
		if zv.Bytes == nil {
			v.Set(reflect.Zero(v.Type()))
			return nil
		}
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		return u.decodeAny(zv, v.Elem())
	case reflect.String:
		// XXX We bundle string, bstring, type, error all into string.
		// See issue #1853.
		switch zng.AliasedType(zv.Type) {
		case zng.TypeString, zng.TypeBstring, zng.TypeType, zng.TypeError:
		default:
			return incompatTypeError(zv.Type, v)
		}
		if zv.Bytes == nil {
			v.Set(reflect.Zero(v.Type()))
			return nil
		}
		x, err := zng.DecodeString(zv.Bytes)
		v.SetString(x)
		return err
	case reflect.Bool:
		if zng.AliasedType(zv.Type) != zng.TypeBool {
			return incompatTypeError(zv.Type, v)
		}
		if zv.Bytes == nil {
			v.Set(reflect.Zero(v.Type()))
			return nil
		}
		x, err := zng.DecodeBool(zv.Bytes)
		v.SetBool(x)
		return err
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch zng.AliasedType(zv.Type) {
		case zng.TypeInt8, zng.TypeInt16, zng.TypeInt32, zng.TypeInt64:
		default:
			return incompatTypeError(zv.Type, v)
		}
		if zv.Bytes == nil {
			v.Set(reflect.Zero(v.Type()))
			return nil
		}
		x, err := zng.DecodeInt(zv.Bytes)
		v.SetInt(x)
		return err
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		switch zng.AliasedType(zv.Type) {
		case zng.TypeUint8, zng.TypeUint16, zng.TypeUint32, zng.TypeUint64:
		default:
			return incompatTypeError(zv.Type, v)
		}
		if zv.Bytes == nil {
			v.Set(reflect.Zero(v.Type()))
			return nil
		}
		x, err := zng.DecodeUint(zv.Bytes)
		v.SetUint(x)
		return err
	case reflect.Float32, reflect.Float64:
		// TODO: zng.TypeFloat32 when it lands
		switch zng.AliasedType(zv.Type) {
		case zng.TypeFloat64:
		default:
			return incompatTypeError(zv.Type, v)
		}
		if zv.Bytes == nil {
			v.Set(reflect.Zero(v.Type()))
			return nil
		}
		x, err := zng.DecodeFloat64(zv.Bytes)
		v.SetFloat(x)
		return err
	default:
		return fmt.Errorf("unsupported type: %v", v.Kind())
	}
}

func (u *UnmarshalContext) decodeIP(zv zng.Value, v reflect.Value) error {
	if zng.AliasedType(zv.Type) != zng.TypeIP {
		return incompatTypeError(zv.Type, v)
	}
	if zv.Bytes == nil {
		v.Set(reflect.Zero(v.Type()))
		return nil
	}
	x, err := zng.DecodeIP(zv.Bytes)
	v.Set(reflect.ValueOf(x))
	return err
}

func (u *UnmarshalContext) decodeRecord(zv zng.Value, sval reflect.Value) error {
	recType, ok := zng.AliasedType(zv.Type).(*zng.TypeRecord)
	if !ok {
		return errors.New("not a record")
	}
	nameToField := make(map[string]int)
	stype := sval.Type()
	for i := 0; i < stype.NumField(); i++ {
		field := stype.Field(i)
		name := fieldName(field)
		nameToField[name] = i
	}
	for i, it := 0, zv.Iter(); !it.Done(); i++ {
		if i >= len(recType.Columns) {
			return zng.ErrMismatch
		}
		itzv, _, err := it.Next()
		if err != nil {
			return err
		}
		name := recType.Columns[i].Name
		if fieldIdx, ok := nameToField[name]; ok {
			typ := recType.Columns[i].Type
			if err := u.decodeAny(zng.Value{typ, itzv}, sval.Field(fieldIdx)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (u *UnmarshalContext) decodeArray(zv zng.Value, arrVal reflect.Value) error {
	arrType, ok := zng.AliasedType(zv.Type).(*zng.TypeArray)
	if !ok {
		return errors.New("not an array")
	}
	if zv.Bytes == nil {
		return nil
	}
	i := 0
	for it := zv.Iter(); !it.Done(); i++ {
		itzv, _, err := it.Next()
		if err != nil {
			return err
		}
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
		if err := u.decodeAny(zng.Value{arrType.Type, itzv}, arrVal.Index(i)); err != nil {
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

type Binding struct {
	Name     string      // user-defined name
	Template interface{} // zero-valued entity used as template for new such objects
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

func (b *binder) enterTemplate(template interface{}) error {
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

func typeOfTemplate(template interface{}) (reflect.Type, error) {
	v := reflect.ValueOf(template)
	if !v.IsValid() {
		return nil, errors.New("invalid template")
	}
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	return v.Type(), nil
}

func typeNameOfValue(value interface{}) (string, error) {
	typ, err := typeOfTemplate(value)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s", typ.PkgPath(), typ.Name()), nil
}

func (u *UnmarshalContext) lookupType(typ zng.Type) (reflect.Type, error) {
	switch typ := typ.(type) {
	case *zng.TypeAlias:
		if template := u.binder.lookup(typ.Name); template != nil {
			return template, nil
		}
		// Ignore aliases for which there are no bindings.
		// If an interface type being marshaled into doesn't
		// have a binding, then a type mismatch will be caught
		// by reflect when the Set() method is called on the
		// value and the concrete value doesn't implement the
		// interface.
		return u.lookupType(typ.Type)
	case *zng.TypeRecord:
		return u.lookupTypeRecord(typ)
	case *zng.TypeArray:
		elemType, err := u.lookupType(typ.Type)
		if err != nil {
			return nil, err
		}
		return reflect.SliceOf(elemType), nil
	case *zng.TypeSet:
		elemType, err := u.lookupType(typ.Type)
		if err != nil {
			return nil, err
		}
		return reflect.SliceOf(elemType), nil
	case *zng.TypeUnion, *zng.TypeEnum:
		// For now just return nil here. The layer above will flag
		// a type error.  At some point, we can create Go-native data structures
		// in package zng for representing a union or enum as a standalone
		// entity.  See issue #1853.
		return nil, nil
	case *zng.TypeMap:
		keyType, err := u.lookupType(typ.KeyType)
		if err != nil {
			return nil, err
		}
		valType, err := u.lookupType(typ.ValType)
		if err != nil {
			return nil, err
		}
		return reflect.MapOf(keyType, valType), nil
	default:
		return u.lookupPrimitiveType(typ)
	}
}

func (u *UnmarshalContext) lookupTypeRecord(typ *zng.TypeRecord) (reflect.Type, error) {
	return nil, errors.New("unmarshaling records into interface value requires type binding")
}

func (u *UnmarshalContext) lookupPrimitiveType(typ zng.Type) (reflect.Type, error) {
	var v interface{}
	switch typ := typ.(type) {
	// XXX We should have counterparts for bstring, error, and type type.
	// See issue #1853.
	case *zng.TypeOfString, *zng.TypeOfBstring, *zng.TypeOfError, *zng.TypeOfType:
		v = ""
	case *zng.TypeOfBool:
		v = false
	case *zng.TypeOfUint8:
		v = uint8(0)
	case *zng.TypeOfUint16:
		v = uint16(0)
	case *zng.TypeOfUint32:
		v = uint32(0)
	case *zng.TypeOfUint64:
		v = uint64(0)
	case *zng.TypeOfInt8:
		v = int8(0)
	case *zng.TypeOfInt16:
		v = int16(0)
	case *zng.TypeOfInt32:
		v = int32(0)
	case *zng.TypeOfInt64:
		v = int64(0)
	// TODO: zng.TypeFloat32 when it lands
	case *zng.TypeOfFloat64:
		v = float64(0)
	case *zng.TypeOfIP:
		v = net.IP{}
	case *zng.TypeOfNet:
		v = net.IPNet{}
	case *zng.TypeOfTime:
		v = time.Time{}
	case *zng.TypeOfDuration:
		v = time.Duration(0)
	case *zng.TypeOfNull:
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown zng type: %v", typ)
	}
	return reflect.TypeOf(v), nil
}
