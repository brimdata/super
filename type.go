// Package bsup implements a data typing system based on the zeek type system.
// All zeek types are defined here and implement the Type interface while instances
// of values implement the Value interface.  All values conform to exactly one type.
// The package provides a fast-path for comparing a value to a byte slice
// without having to create a zeek value from the byte slice.  To exploit this,
// all values include a Comparison method that returns a Predicate function that
// takes a byte slice and a Type and returns a boolean indicating whether the
// the byte slice with the indicated Type matches the value.  The package also
// provides mechanism for coercing values in well-defined and natural ways.
package super

import (
	"cmp"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/brimdata/super/zcode"
)

var (
	ErrNotArray  = errors.New("cannot index a non-array")
	ErrIndex     = errors.New("array index out of bounds")
	ErrUnionTag  = errors.New("invalid union tag")
	ErrEnumIndex = errors.New("enum index out of bounds")
)

// A Type is an interface presented by a zeek type.
// Types can be used to infer type compatibility and create new values
// of the underlying type.
type Type interface {
	// ID returns a unique (per Context) identifier that
	// represents this type.  For a named type, this identifier
	// represents the underlying type and not the named type itself.
	// Callers that care about the underlying type of a Value for
	// example should prefer to use this instead of using a Go
	// type assertion on a Type instance.
	ID() int
	Kind() Kind
}

type Kind int

const (
	PrimitiveKind Kind = iota
	RecordKind
	ArrayKind
	SetKind
	MapKind
	UnionKind
	EnumKind
	ErrorKind
)

func (k Kind) String() string {
	switch k {
	case PrimitiveKind:
		return "primitive"
	case RecordKind:
		return "record"
	case ArrayKind:
		return "array"
	case SetKind:
		return "set"
	case MapKind:
		return "map"
	case UnionKind:
		return "union"
	case EnumKind:
		return "enum"
	case ErrorKind:
		return "error"
	default:
		return fmt.Sprintf("<unknown kind: %d>", k)
	}
}

var (
	TypeUint8    = &TypeOfUint8{}
	TypeUint16   = &TypeOfUint16{}
	TypeUint32   = &TypeOfUint32{}
	TypeUint64   = &TypeOfUint64{}
	TypeInt8     = &TypeOfInt8{}
	TypeInt16    = &TypeOfInt16{}
	TypeInt32    = &TypeOfInt32{}
	TypeInt64    = &TypeOfInt64{}
	TypeDuration = &TypeOfDuration{}
	TypeTime     = &TypeOfTime{}
	TypeFloat16  = &TypeOfFloat16{}
	TypeFloat32  = &TypeOfFloat32{}
	TypeFloat64  = &TypeOfFloat64{}
	// XXX add TypeDecimal
	TypeBool   = &TypeOfBool{}
	TypeBytes  = &TypeOfBytes{}
	TypeString = &TypeOfString{}
	TypeIP     = &TypeOfIP{}
	TypeNet    = &TypeOfNet{}
	TypeType   = &TypeOfType{}
	TypeNull   = &TypeOfNull{}
)

// Primary Type IDs

const (
	IDUint8       = 0
	IDUint16      = 1
	IDUint32      = 2
	IDUint64      = 3
	IDUint128     = 4
	IDUint256     = 5
	IDInt8        = 6
	IDInt16       = 7
	IDInt32       = 8
	IDInt64       = 9
	IDInt128      = 10
	IDInt256      = 11
	IDDuration    = 12
	IDTime        = 13
	IDFloat16     = 14
	IDFloat32     = 15
	IDFloat64     = 16
	IDFloat128    = 17
	IDFloat256    = 18
	IDDecimal32   = 19
	IDDecimal64   = 20
	IDDecimal128  = 21
	IDDecimal256  = 22
	IDBool        = 23
	IDBytes       = 24
	IDString      = 25
	IDIP          = 26
	IDNet         = 27
	IDType        = 28
	IDNull        = 29
	IDTypeComplex = 30
)

// Encodings for complex type values.

const (
	TypeValueRecord  = 30
	TypeValueArray   = 31
	TypeValueSet     = 32
	TypeValueMap     = 33
	TypeValueUnion   = 34
	TypeValueEnum    = 35
	TypeValueError   = 36
	TypeValueNameDef = 37
	TypeValueNameRef = 38
	TypeValueMax     = TypeValueNameRef
)

// True iff the type id is encoded as a BSUP signed or unsigened integer zcode.Bytes.
func IsInteger(id int) bool {
	return id <= IDInt256
}

// True iff the type id is encoded as a BSUP signed or unsigned integer zcode.Bytes,
// float16 zcode.Bytes, float32 zcode.Bytes, or float64 zcode.Bytes.
func IsNumber(id int) bool {
	return id <= IDDecimal256
}

// True iff the type id is encoded as a float encoding.
// XXX add IDDecimal here when we implement coercible math with it.
func IsFloat(id int) bool {
	return id >= IDFloat16 && id <= IDFloat256
}

// True iff the type id is encoded as a number encoding and is signed.
func IsSigned(id int) bool {
	return id >= IDInt8 && id <= IDTime
}

// True iff the type id is encoded as a number encoding and is unsigned.
func IsUnsigned(id int) bool {
	return id <= IDUint256
}

func LookupPrimitive(name string) Type {
	switch name {
	case "uint8":
		return TypeUint8
	case "uint16":
		return TypeUint16
	case "uint32":
		return TypeUint32
	case "uint64":
		return TypeUint64
	case "int8":
		return TypeInt8
	case "int16":
		return TypeInt16
	case "int32":
		return TypeInt32
	case "int64":
		return TypeInt64
	case "duration":
		return TypeDuration
	case "time":
		return TypeTime
	case "float16":
		return TypeFloat16
	case "float32":
		return TypeFloat32
	case "float64":
		return TypeFloat64
	case "bool":
		return TypeBool
	case "bytes":
		return TypeBytes
	case "string":
		return TypeString
	case "ip":
		return TypeIP
	case "net":
		return TypeNet
	case "type":
		return TypeType
	case "null":
		return TypeNull
	}
	return nil
}

func PrimitiveName(typ Type) string {
	switch typ.(type) {
	case *TypeOfUint8:
		return "uint8"
	case *TypeOfUint16:
		return "uint16"
	case *TypeOfUint32:
		return "uint32"
	case *TypeOfUint64:
		return "uint64"
	case *TypeOfInt8:
		return "int8"
	case *TypeOfInt16:
		return "int16"
	case *TypeOfInt32:
		return "int32"
	case *TypeOfInt64:
		return "int64"
	case *TypeOfDuration:
		return "duration"
	case *TypeOfTime:
		return "time"
	case *TypeOfFloat16:
		return "float16"
	case *TypeOfFloat32:
		return "float32"
	case *TypeOfFloat64:
		return "float64"
	case *TypeOfBool:
		return "bool"
	case *TypeOfBytes:
		return "bytes"
	case *TypeOfString:
		return "string"
	case *TypeOfIP:
		return "ip"
	case *TypeOfNet:
		return "net"
	case *TypeOfType:
		return "type"
	case *TypeOfNull:
		return "null"
	default:
		return fmt.Sprintf("unknown primitive type: %T", typ)
	}
}

func LookupPrimitiveByID(id int) (Type, error) {
	if id < 0 {
		return nil, fmt.Errorf("negative type ID: %d", id)
	}
	if id >= IDTypeComplex {
		return nil, fmt.Errorf("type ID too large for primitive: %d", id)
	}
	switch id {
	case IDBool:
		return TypeBool, nil
	case IDInt8:
		return TypeInt8, nil
	case IDUint8:
		return TypeUint8, nil
	case IDInt16:
		return TypeInt16, nil
	case IDUint16:
		return TypeUint16, nil
	case IDInt32:
		return TypeInt32, nil
	case IDUint32:
		return TypeUint32, nil
	case IDInt64:
		return TypeInt64, nil
	case IDUint64:
		return TypeUint64, nil
	case IDFloat16:
		return TypeFloat16, nil
	case IDFloat32:
		return TypeFloat32, nil
	case IDFloat64:
		return TypeFloat64, nil
	case IDBytes:
		return TypeBytes, nil
	case IDString:
		return TypeString, nil
	case IDIP:
		return TypeIP, nil
	case IDNet:
		return TypeNet, nil
	case IDTime:
		return TypeTime, nil
	case IDDuration:
		return TypeDuration, nil
	case IDType:
		return TypeType, nil
	case IDNull:
		return TypeNull, nil
	}
	return nil, fmt.Errorf("primitive type ID %d not implemented", id)
}

// Utilities shared by complex types (ie, set and array)

// InnerType returns the element type for the underlying set or array type or
// nil if the underlying type is not a set or array.
func InnerType(typ Type) Type {
	switch typ := TypeUnder(typ).(type) {
	case *TypeSet:
		return typ.Type
	case *TypeArray:
		return typ.Type
	default:
		return nil
	}
}

func IsUnionType(typ Type) bool {
	_, ok := TypeUnder(typ).(*TypeUnion)
	return ok
}

func IsRecordType(typ Type) bool {
	_, ok := TypeUnder(typ).(*TypeRecord)
	return ok
}

func TypeRecordOf(typ Type) *TypeRecord {
	t, _ := TypeUnder(typ).(*TypeRecord)
	return t
}

func IsContainerType(typ Type) bool {
	switch typ := typ.(type) {
	case *TypeNamed:
		return IsContainerType(typ.Type)
	case *TypeSet, *TypeArray, *TypeRecord, *TypeUnion, *TypeMap:
		return true
	default:
		return false
	}
}

func IsPrimitiveType(typ Type) bool {
	return !IsContainerType(typ)
}

func TypeID(typ Type) int {
	if named, ok := typ.(*TypeNamed); ok {
		return named.id
	}
	return typ.ID()
}

// UniqueTypes returns the set of unique Types in types in sorted
// order. types will be sorted and deduplicated in place.
func UniqueTypes(types []Type) []Type {
	sort.SliceStable(types, func(i, j int) bool {
		return CompareTypes(types[i], types[j]) < 0
	})
	out := types[:0]
	var prev Type
	for _, typ := range types {
		if typ != prev {
			out = append(out, typ)
			prev = typ
		}
	}
	return out
}

func CompareTypes(a, b Type) int {
	aID, bID := a.ID(), b.ID()
	if aID == bID {
		if a, ok := a.(*TypeNamed); ok {
			if b, ok := b.(*TypeNamed); ok {
				// Named types sharing an underlying type are
				// ordered by name.
				return strings.Compare(a.Name, b.Name)
			}
			// Named type a is ordered after its underlying type b.
			return 1
		}
		if _, ok := b.(*TypeNamed); ok {
			// Named type b is ordered after its underlying type a.
			return -1
		}
		// a == b
		return 0
	}
	if cmp := cmp.Compare(a.Kind(), b.Kind()); cmp != 0 {
		return cmp
	}
	a, b = TypeUnder(a), TypeUnder(b)
	switch a.Kind() {
	case PrimitiveKind:
		return cmp.Compare(aID, bID)
	case RecordKind:
		ra, rb := TypeRecordOf(a), TypeRecordOf(b)
		// First compare number of fields.
		if cmp := cmp.Compare(len(ra.Fields), len(rb.Fields)); cmp != 0 {
			return cmp
		}
		// Second compare field names.
		for i := range ra.Fields {
			if cmp := strings.Compare(ra.Fields[i].Name, rb.Fields[i].Name); cmp != 0 {
				return cmp
			}
		}
		// Lastly compare field types.
		for i := range ra.Fields {
			if cmp := CompareTypes(ra.Fields[i].Type, rb.Fields[i].Type); cmp != 0 {
				return cmp
			}
		}
		return 0
	case ArrayKind, SetKind:
		a, b = InnerType(a), InnerType(b)
		return CompareTypes(a, b)
	case MapKind:
		ma, mb := a.(*TypeMap), b.(*TypeMap)
		if cmp := CompareTypes(ma.KeyType, mb.KeyType); cmp != 0 {
			return cmp
		}
		return CompareTypes(ma.ValType, mb.ValType)
	case UnionKind:
		ua, ub := a.(*TypeUnion), b.(*TypeUnion)
		if cmp := cmp.Compare(len(ua.Types), len(ub.Types)); cmp != 0 {
			return cmp
		}
		for i := range ua.Types {
			if cmp := CompareTypes(ua.Types[i], ub.Types[i]); cmp != 0 {
				return cmp
			}
		}
		return 0
	case EnumKind:
		ea, eb := a.(*TypeEnum), b.(*TypeEnum)
		if cmp := cmp.Compare(len(ea.Symbols), len(eb.Symbols)); cmp != 0 {
			return cmp
		}
		for i := range ea.Symbols {
			if cmp := strings.Compare(ea.Symbols[i], eb.Symbols[i]); cmp != 0 {
				return cmp
			}
		}
		return 0
	case ErrorKind:
		ea, eb := a.(*TypeError), b.(*TypeError)
		return CompareTypes(ea.Type, eb.Type)
	}
	return 0
}

type TypeOfType struct{}

func (t *TypeOfType) ID() int {
	return IDType
}

func (t *TypeOfType) Kind() Kind {
	return PrimitiveKind
}

func EncodeTypeValue(t Type) zcode.Bytes {
	return AppendTypeValue(nil, t)
}

func AppendTypeValue(b zcode.Bytes, t Type) zcode.Bytes {
	var typedefs map[string]Type
	return appendTypeValue(b, t, &typedefs)
}

func appendTypeValue(b zcode.Bytes, t Type, typedefs *map[string]Type) zcode.Bytes {
	switch t := t.(type) {
	case *TypeNamed:
		if *typedefs == nil {
			*typedefs = make(map[string]Type)
		}
		id := byte(TypeValueNameDef)
		if previous := (*typedefs)[t.Name]; previous == t.Type {
			id = TypeValueNameRef
		}
		b = append(b, id)
		b = binary.AppendUvarint(b, uint64(len(t.Name)))
		b = append(b, zcode.Bytes(t.Name)...)
		if id == TypeValueNameRef {
			return b
		}
		b = appendTypeValue(b, t.Type, typedefs)
		// Set the typedef *after* the child has been recursively traversed
		// in case the child sets the name to a different type.  This insures
		// that the DFS binding order is maintained.
		(*typedefs)[t.Name] = t.Type
		return b

	case *TypeRecord:
		b = append(b, TypeValueRecord)
		b = binary.AppendUvarint(b, uint64(len(t.Fields)))
		for _, f := range t.Fields {
			b = binary.AppendUvarint(b, uint64(len(f.Name)))
			b = append(b, f.Name...)
			b = appendTypeValue(b, f.Type, typedefs)
		}
		return b
	case *TypeUnion:
		b = append(b, TypeValueUnion)
		b = binary.AppendUvarint(b, uint64(len(t.Types)))
		for _, t := range t.Types {
			b = appendTypeValue(b, t, typedefs)
		}
		return b
	case *TypeSet:
		b = append(b, TypeValueSet)
		return appendTypeValue(b, t.Type, typedefs)
	case *TypeArray:
		b = append(b, TypeValueArray)
		return appendTypeValue(b, t.Type, typedefs)
	case *TypeEnum:
		b = append(b, TypeValueEnum)
		b = binary.AppendUvarint(b, uint64(len(t.Symbols)))
		for _, s := range t.Symbols {
			b = binary.AppendUvarint(b, uint64(len(s)))
			b = append(b, s...)
		}
		return b
	case *TypeMap:
		b = append(b, TypeValueMap)
		b = appendTypeValue(b, t.KeyType, typedefs)
		return appendTypeValue(b, t.ValType, typedefs)
	case *TypeError:
		b = append(b, TypeValueError)
		return appendTypeValue(b, t.Type, typedefs)
	default:
		// Primitive type
		return append(b, byte(t.ID()))
	}
}
