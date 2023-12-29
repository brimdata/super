package zed

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"net/netip"
	"runtime/debug"
	"unsafe"

	"github.com/brimdata/zed/pkg/field"
	"github.com/brimdata/zed/pkg/nano"
	"github.com/brimdata/zed/zcode"
)

var (
	ErrMissingField = errors.New("record missing a field")
	ErrNotContainer = errors.New("expected container type, got primitive")
)

var (
	NullUint8    = &Value{typ: TypeUint8}
	NullUint16   = &Value{typ: TypeUint16}
	NullUint32   = &Value{typ: TypeUint32}
	NullUint64   = &Value{typ: TypeUint64}
	NullInt8     = &Value{typ: TypeInt8}
	NullInt16    = &Value{typ: TypeInt16}
	NullInt32    = &Value{typ: TypeInt32}
	NullInt64    = &Value{typ: TypeInt64}
	NullDuration = &Value{typ: TypeDuration}
	NullTime     = &Value{typ: TypeTime}
	NullFloat16  = &Value{typ: TypeFloat16}
	NullFloat32  = &Value{typ: TypeFloat32}
	NullFloat64  = &Value{typ: TypeFloat64}
	NullBool     = &Value{typ: TypeBool}
	NullBytes    = &Value{typ: TypeBytes}
	NullString   = &Value{typ: TypeString}
	NullIP       = &Value{typ: TypeIP}
	NullNet      = &Value{typ: TypeNet}
	NullType     = &Value{typ: TypeType}
	Null         = &Value{typ: TypeNull}

	False = NewBool(false)
	True  = NewBool(true)
)

type Allocator interface {
	NewValue(Type, zcode.Bytes) *Value
	CopyValue(Value) *Value
}

type Value struct {
	typ Type
	// If base == &nativeBase, len holds this Value's native representation.
	// Otherwise, unsafe.Slice(base, len) holds its ZNG representation.
	base *byte
	len  uint64
}

func (v *Value) Type() Type { return v.typ }

func NewValue(t Type, b zcode.Bytes) *Value { return &Value{t, unsafe.SliceData(b), uint64(len(b))} }
func (v *Value) bytes() zcode.Bytes         { return unsafe.Slice(v.base, v.len) }

// nativeBase is the base address for all native Values, which are encoded with
// the base field set to this address and the len field set to the bits of the
// Value's native representation.
var nativeBase byte

func newNativeValue(t Type, x uint64) *Value { return &Value{t, &nativeBase, x} }
func (v *Value) native() (uint64, bool)      { return v.len, v.base == &nativeBase }

func NewUint(t Type, u uint64) *Value    { return newNativeValue(t, u) }
func NewUint8(u uint8) *Value            { return newNativeValue(TypeUint8, uint64(u)) }
func NewUint16(u uint16) *Value          { return newNativeValue(TypeUint16, uint64(u)) }
func NewUint32(u uint32) *Value          { return newNativeValue(TypeUint32, uint64(u)) }
func NewUint64(u uint64) *Value          { return newNativeValue(TypeUint64, u) }
func NewInt(t Type, i int64) *Value      { return newNativeValue(t, uint64(i)) }
func NewInt8(i int8) *Value              { return newNativeValue(TypeInt8, uint64(i)) }
func NewInt16(i int16) *Value            { return newNativeValue(TypeInt16, uint64(i)) }
func NewInt32(i int32) *Value            { return newNativeValue(TypeInt32, uint64(i)) }
func NewInt64(i int64) *Value            { return newNativeValue(TypeInt64, uint64(i)) }
func NewDuration(d nano.Duration) *Value { return newNativeValue(TypeDuration, uint64(d)) }
func NewTime(ts nano.Ts) *Value          { return newNativeValue(TypeTime, uint64(ts)) }
func NewFloat(t Type, f float64) *Value  { return newNativeValue(t, math.Float64bits(f)) }
func NewFloat16(f float32) *Value        { return newNativeValue(TypeFloat16, math.Float64bits(float64(f))) }
func NewFloat32(f float32) *Value        { return newNativeValue(TypeFloat32, math.Float64bits(float64(f))) }
func NewFloat64(f float64) *Value        { return newNativeValue(TypeFloat64, math.Float64bits(f)) }
func NewBool(b bool) *Value              { return newNativeValue(TypeBool, boolToUint64(b)) }
func NewBytes(b []byte) *Value           { return NewValue(TypeBytes, b) }
func NewString(s string) *Value          { return &Value{TypeString, unsafe.StringData(s), uint64(len(s))} }
func NewIP(a netip.Addr) *Value          { return NewValue(TypeIP, EncodeIP(a)) }
func NewNet(p netip.Prefix) *Value       { return NewValue(TypeNet, EncodeNet(p)) }
func NewTypeValue(t Type) *Value         { return NewValue(TypeNet, EncodeTypeValue(t)) }

func boolToUint64(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}

// Uint returns v's underlying value.  It panics if v's underlying type is not
// TypeUint8, TypeUint16, TypeUint32, or TypeUint64.
func (v *Value) Uint() uint64 {
	if v.Type().ID() > IDUint64 {
		panic(fmt.Sprintf("zed.Value.Uint called on %T", v.Type()))
	}
	if x, ok := v.native(); ok {
		return x
	}
	return DecodeUint(v.bytes())
}

// Int returns v's underlying value.  It panics if v's underlying type is not
// TypeInt8, TypeInt16, TypeInt32, TypeInt64, TypeDuration, or TypeTime.
func (v *Value) Int() int64 {
	if !IsSigned(v.Type().ID()) {
		panic(fmt.Sprintf("zed.Value.Int called on %T", v.Type()))
	}
	if x, ok := v.native(); ok {
		return int64(x)
	}
	return DecodeInt(v.bytes())
}

// Float returns v's underlying value.  It panics if v's underlying type is not
// TypeFloat16, TypeFloat32, or TypeFloat64.
func (v *Value) Float() float64 {
	if !IsFloat(v.Type().ID()) {
		panic(fmt.Sprintf("zed.Value.Float called on %T", v.Type()))
	}
	if x, ok := v.native(); ok {
		return math.Float64frombits(x)
	}
	return DecodeFloat(v.bytes())
}

// Bool returns v's underlying value.  It panics if v's underlying type is not
// TypeBool.
func (v *Value) Bool() bool {
	if v.Type().ID() != IDBool {
		panic(fmt.Sprintf("zed.Value.Bool called on %T", v.Type()))
	}
	if x, ok := v.native(); ok {
		return x != 0
	}
	return DecodeBool(v.bytes())
}

// Bytes returns v's ZNG representation.
func (v *Value) Bytes() zcode.Bytes {
	if x, ok := v.native(); ok {
		switch v.Type().ID() {
		case IDUint8, IDUint16, IDUint32, IDUint64:
			return EncodeUint(x)
		case IDInt8, IDInt16, IDInt32, IDInt64, IDDuration, IDTime:
			return EncodeInt(int64(x))
		case IDFloat16:
			return EncodeFloat16(float32(math.Float64frombits(x)))
		case IDFloat32:
			return EncodeFloat32(float32(math.Float64frombits(x)))
		case IDFloat64:
			return EncodeFloat64(math.Float64frombits(x))
		case IDBool:
			return EncodeBool(x != 0)
		}
		panic(v.Type())
	}
	return v.bytes()
}

func (v *Value) IsContainer() bool {
	return IsContainerType(v.Type())
}

// String implements fmt.Stringer.String.  It should only be used for logs,
// debugging, etc.  Any caller that requires a specific output format should use
// FormatAs() instead.
func (v *Value) String() string {
	return fmt.Sprintf("%s: %s", v.Type(), v.Encode(nil))
}

// Encode appends the ZNG representation of this value to the passed in
// argument and returns the resulting zcode.Bytes (which may or may not
// be the same underlying buffer, as with append(), depending on its capacity)
func (v *Value) Encode(dst zcode.Bytes) zcode.Bytes {
	//XXX don't need this...
	return zcode.Append(dst, v.Bytes())
}

func (v *Value) Iter() zcode.Iter {
	return v.Bytes().Iter()
}

// If the passed-in element is an array, attempt to get the idx'th
// element, and return its type and raw representation.  Returns an
// error if the passed-in element is not an array or if idx is
// outside the array bounds.
func (v *Value) ArrayIndex(idx int64) (Value, error) {
	vec, ok := v.Type().(*TypeArray)
	if !ok {
		return Value{}, ErrNotArray
	}
	if idx < 0 {
		return Value{}, ErrIndex
	}
	for i, it := 0, v.Iter(); !it.Done(); i++ {
		bytes := it.Next()
		if i == int(idx) {
			return *NewValue(vec.Type, bytes), nil
		}
	}
	return Value{}, ErrIndex
}

// Elements returns an array of Values for the given container type.
// Returns an error if the element is not an array or set.
func (v *Value) Elements() ([]Value, error) {
	innerType := InnerType(v.Type())
	if innerType == nil {
		return nil, ErrNotContainer
	}
	var elements []Value
	for it := v.Iter(); !it.Done(); {
		elements = append(elements, *NewValue(innerType, it.Next()))
	}
	return elements, nil
}

func (v *Value) ContainerLength() (int, error) {
	switch v.Type().(type) {
	case *TypeSet, *TypeArray:
		if v.IsNull() {
			return 0, nil
		}
		var n int
		for it := v.Iter(); !it.Done(); {
			it.Next()
			n++
		}
		return n, nil
	case *TypeMap:
		if v.IsNull() {
			return 0, nil
		}
		var n int
		for it := v.Iter(); !it.Done(); {
			it.Next()
			it.Next()
			n++
		}
		return n, nil
	default:
		return -1, ErrNotContainer
	}
}

// IsNull returns true if and only if v is a null value of any type.
func (v *Value) IsNull() bool {
	return v.base == nil
}

// Copy returns a copy of v that shares no storage.
func (v *Value) Copy() *Value {
	if x, ok := v.native(); ok {
		return newNativeValue(v.Type(), x)
	}
	return NewValue(v.Type(), bytes.Clone(v.bytes()))
}

// CopyFrom copies from into v, reusing v's storage if possible.
func (v *Value) CopyFrom(from *Value) {
	if _, ok := from.native(); ok || from.IsNull() {
		*v = *from
	} else if _, ok := v.native(); ok || v.IsNull() || v.len < from.len {
		*v = *NewValue(from.Type(), bytes.Clone(from.bytes()))
	} else {
		*v = *NewValue(from.Type(), append(v.bytes()[:0], from.bytes()...))
	}
}

func (v *Value) IsString() bool {
	_, ok := TypeUnder(v.Type()).(*TypeOfString)
	return ok
}

func (v *Value) IsError() bool {
	_, ok := TypeUnder(v.Type()).(*TypeError)
	return ok
}

func (v *Value) IsMissing() bool {
	if v == nil {
		return true
	}
	if typ, ok := v.Type().(*TypeError); ok {
		return typ.IsMissing(v.Bytes())
	}
	return false
}

func (v *Value) IsQuiet() bool {
	if typ, ok := v.Type().(*TypeError); ok {
		return typ.IsQuiet(v.Bytes())
	}
	return false
}

// Equal reports whether p and v have the same type and the same ZNG
// representation.
func (v *Value) Equal(p Value) bool {
	if v.Type() != p.Type() {
		return false
	}
	if x, ok := v.native(); ok {
		if y, ok := p.native(); ok {
			return x == y
		}
	}
	return bytes.Equal(v.Bytes(), p.Bytes())
}

func (r *Value) HasField(field string) bool {
	return TypeRecordOf(r.Type()).HasField(field)
}

// Walk traverses a value in depth-first order, calling a
// Visitor on the way.
func (r *Value) Walk(rv Visitor) error {
	return Walk(r.Type(), r.Bytes(), rv)
}

func (r *Value) nth(n int) zcode.Bytes {
	var zv zcode.Bytes
	for i, it := 0, r.Bytes().Iter(); i <= n; i++ {
		if it.Done() {
			return nil
		}
		zv = it.Next()
	}
	return zv
}

func (r *Value) Fields() []Field {
	return TypeRecordOf(r.Type()).Fields
}

func (v *Value) DerefByColumn(col int) *Value {
	if v != nil {
		if bytes := v.nth(col); bytes != nil {
			v = NewValue(v.Fields()[col].Type, bytes)
		} else {
			v = nil
		}
	}
	return v
}

func (v *Value) IndexOfField(field string) (int, bool) {
	if typ := TypeRecordOf(v.Type()); typ != nil {
		return typ.IndexOfField(field)
	}
	return 0, false
}

func (v *Value) Deref(field string) *Value {
	if v == nil {
		return nil
	}
	i, ok := v.IndexOfField(field)
	if !ok {
		return nil
	}
	return v.DerefByColumn(i)
}

func (v *Value) DerefPath(path field.Path) *Value {
	for len(path) != 0 {
		v = v.Deref(path[0])
		path = path[1:]
	}
	return v
}

func (v *Value) AsString() string {
	if v != nil && TypeUnder(v.Type()) == TypeString {
		return DecodeString(v.Bytes())
	}
	return ""
}

// AsBool returns v's underlying value.  It returns false if v is nil or v's
// underlying type is not TypeBool.
func (v *Value) AsBool() bool {
	if v != nil && TypeUnder(v.Type()) == TypeBool {
		return v.Bool()
	}
	return false
}

func (v *Value) AsInt() int64 {
	if v != nil {
		switch TypeUnder(v.Type()).(type) {
		case *TypeOfUint8, *TypeOfUint16, *TypeOfUint32, *TypeOfUint64:
			return int64(v.Uint())
		case *TypeOfInt8, *TypeOfInt16, *TypeOfInt32, *TypeOfInt64:
			return v.Int()
		}
	}
	return 0
}

func (v *Value) AsTime() nano.Ts {
	if v != nil && TypeUnder(v.Type()) == TypeTime {
		return DecodeTime(v.Bytes())
	}
	return 0
}

func (v *Value) MissingAsNull() *Value {
	if v.IsMissing() {
		return Null
	}
	return v
}

// Under resolves named types and untags unions repeatedly, returning a value
// guaranteed to have neither a named type nor a union type.  When Under returns
// a new value (i.e., one that differs from v), it uses dst if not nil.
// Otherwise, Under allocates a new value.
func (v *Value) Under(dst *Value) *Value {
	switch v.Type().(type) {
	case *TypeUnion, *TypeNamed:
		return v.under(dst)
	}
	// This is the common case; make sure the compiler can inline it.
	return v
}

// under contains logic for Under that the compiler won't inline.
func (v *Value) under(dst *Value) *Value {
	typ, bytes := v.Type(), v.Bytes()
	for {
		typ = TypeUnder(typ)
		union, ok := typ.(*TypeUnion)
		if !ok {
			if dst == nil {
				return NewValue(typ, bytes)
			}
			*dst = *NewValue(typ, bytes)
			return dst
		}
		typ, bytes = union.Untag(bytes)
	}
}

// Validate checks that v.Bytes is structurally consistent
// with v.Type.  It does not check that the actual leaf
// values when parsed are type compatible with the leaf types.
func (v *Value) Validate() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %+v\n%s", r, debug.Stack())
		}
	}()
	return v.Walk(func(typ Type, body zcode.Bytes) error {
		if typset, ok := typ.(*TypeSet); ok {
			if err := checkSet(typset, body); err != nil {
				return err
			}
			return SkipContainer
		}
		if typ, ok := typ.(*TypeEnum); ok {
			if err := checkEnum(typ, body); err != nil {
				return err
			}
			return SkipContainer
		}
		return nil
	})
}

func checkSet(typ *TypeSet, body zcode.Bytes) error {
	if body == nil {
		return nil
	}
	it := body.Iter()
	var prev zcode.Bytes
	for !it.Done() {
		tagAndBody := it.NextTagAndBody()
		if prev != nil {
			switch bytes.Compare(prev, tagAndBody) {
			case 0:
				return errors.New("invalid ZNG: duplicate set element")
			case 1:
				return errors.New("invalid ZNG: set elements not sorted")
			}
		}
		prev = tagAndBody
	}
	return nil
}

func checkEnum(typ *TypeEnum, body zcode.Bytes) error {
	if body == nil {
		return nil
	}
	if selector := DecodeUint(body); int(selector) >= len(typ.Symbols) {
		return errors.New("enum selector out of range")
	}
	return nil
}
