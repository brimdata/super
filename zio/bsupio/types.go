package bsupio

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"github.com/brimdata/super"
)

const (
	TypeDefRecord = 0
	TypeDefArray  = 1
	TypeDefSet    = 2
	TypeDefMap    = 3
	TypeDefUnion  = 4
	TypeDefEnum   = 5
	TypeDefError  = 6
	TypeDefName   = 7
)

type Encoder struct {
	sctx    *super.Context
	encoded map[super.Type]super.Type
	bytes   []byte
}

func NewEncoder() *Encoder {
	return &Encoder{
		sctx:    super.NewContext(),
		encoded: make(map[super.Type]super.Type),
	}
}

func (e *Encoder) Reset() {
	e.bytes = e.bytes[:0]
	e.encoded = make(map[super.Type]super.Type)
	e.sctx.Reset()
}

func (e *Encoder) Flush() {
	e.bytes = e.bytes[:0]
}

func (e *Encoder) Lookup(external super.Type) super.Type {
	return e.encoded[external]
}

// Encode takes a type from outside this context and constructs a type from
// inside this context and emits BSON typedefs for any type needed to construct
// the new type into the buffer provided.
func (e *Encoder) Encode(external super.Type) (super.Type, error) {
	if typ, ok := e.encoded[external]; ok {
		return typ, nil
	}
	internal, err := e.encode(external)
	if err != nil {
		return nil, err
	}
	e.encoded[external] = internal
	return internal, err
}

func (e *Encoder) encode(ext super.Type) (super.Type, error) {
	switch ext := ext.(type) {
	case *super.TypeRecord:
		return e.encodeTypeRecord(ext)
	case *super.TypeSet:
		return e.encodeTypeSet(ext)
	case *super.TypeArray:
		return e.encodeTypeArray(ext)
	case *super.TypeUnion:
		return e.encodeTypeUnion(ext)
	case *super.TypeMap:
		return e.encodeTypeMap(ext)
	case *super.TypeEnum:
		return e.encodeTypeEnum(ext)
	case *super.TypeNamed:
		return e.encodeTypeName(ext)
	case *super.TypeError:
		return e.encodeTypeError(ext)
	default:
		return ext, nil
	}
}

func (e *Encoder) encodeTypeRecord(ext *super.TypeRecord) (super.Type, error) {
	var fields []super.Field
	for _, f := range ext.Fields {
		child, err := e.Encode(f.Type)
		if err != nil {
			return nil, err
		}
		fields = append(fields, super.NewField(f.Name, child))
	}
	typ, err := e.sctx.LookupTypeRecord(fields)
	if err != nil {
		return nil, err
	}
	e.bytes = append(e.bytes, TypeDefRecord)
	e.bytes = binary.AppendUvarint(e.bytes, uint64(len(fields)))
	for _, f := range fields {
		e.bytes = binary.AppendUvarint(e.bytes, uint64(len(f.Name)))
		e.bytes = append(e.bytes, f.Name...)
		e.bytes = binary.AppendUvarint(e.bytes, uint64(super.TypeID(f.Type)))
	}
	return typ, nil
}

func (e *Encoder) encodeTypeUnion(ext *super.TypeUnion) (super.Type, error) {
	var types []super.Type
	for _, t := range ext.Types {
		t, err := e.Encode(t)
		if err != nil {
			return nil, err
		}
		types = append(types, t)
	}
	typ := e.sctx.LookupTypeUnion(types)
	e.bytes = append(e.bytes, TypeDefUnion)
	e.bytes = binary.AppendUvarint(e.bytes, uint64(len(types)))
	for _, t := range types {
		e.bytes = binary.AppendUvarint(e.bytes, uint64(super.TypeID(t)))
	}
	return typ, nil
}

func (e *Encoder) encodeTypeSet(ext *super.TypeSet) (*super.TypeSet, error) {
	inner, err := e.Encode(ext.Type)
	if err != nil {
		return nil, err
	}
	typ := e.sctx.LookupTypeSet(inner)
	e.bytes = append(e.bytes, TypeDefSet)
	e.bytes = binary.AppendUvarint(e.bytes, uint64(super.TypeID(inner)))
	return typ, nil
}

func (e *Encoder) encodeTypeArray(ext *super.TypeArray) (*super.TypeArray, error) {
	inner, err := e.Encode(ext.Type)
	if err != nil {
		return nil, err
	}
	typ := e.sctx.LookupTypeArray(inner)
	e.bytes = append(e.bytes, TypeDefArray)
	e.bytes = binary.AppendUvarint(e.bytes, uint64(super.TypeID(inner)))
	return typ, nil
}

func (e *Encoder) encodeTypeEnum(ext *super.TypeEnum) (*super.TypeEnum, error) {
	symbols := ext.Symbols
	typ := e.sctx.LookupTypeEnum(symbols)
	e.bytes = append(e.bytes, TypeDefEnum)
	e.bytes = binary.AppendUvarint(e.bytes, uint64(len(symbols)))
	for _, s := range symbols {
		e.bytes = binary.AppendUvarint(e.bytes, uint64(len(s)))
		e.bytes = append(e.bytes, s...)
	}
	return typ, nil
}

func (e *Encoder) encodeTypeMap(ext *super.TypeMap) (*super.TypeMap, error) {
	keyType, err := e.Encode(ext.KeyType)
	if err != nil {
		return nil, err
	}
	valType, err := e.Encode(ext.ValType)
	if err != nil {
		return nil, err
	}
	typ := e.sctx.LookupTypeMap(keyType, valType)
	e.bytes = append(e.bytes, TypeDefMap)
	e.bytes = binary.AppendUvarint(e.bytes, uint64(super.TypeID(keyType)))
	e.bytes = binary.AppendUvarint(e.bytes, uint64(super.TypeID(valType)))
	return typ, nil
}

func (e *Encoder) encodeTypeName(ext *super.TypeNamed) (*super.TypeNamed, error) {
	inner, err := e.Encode(ext.Type)
	if err != nil {
		return nil, err
	}
	typ, err := e.sctx.LookupTypeNamed(ext.Name, inner)
	if err != nil {
		return nil, err
	}
	e.bytes = append(e.bytes, TypeDefName)
	e.bytes = binary.AppendUvarint(e.bytes, uint64(len(typ.Name)))
	e.bytes = append(e.bytes, typ.Name...)
	e.bytes = binary.AppendUvarint(e.bytes, uint64(super.TypeID(typ.Type)))
	return typ, nil
}

func (e *Encoder) encodeTypeError(ext *super.TypeError) (*super.TypeError, error) {
	inner, err := e.Encode(ext.Type)
	if err != nil {
		return nil, err
	}
	typ := e.sctx.LookupTypeError(inner)
	e.bytes = append(e.bytes, TypeDefError)
	e.bytes = binary.AppendUvarint(e.bytes, uint64(super.TypeID(typ.Type)))
	return typ, nil
}

type Decoder struct {
	// shared/output context
	sctx *super.Context
	// Local type IDs are mapped to the shared-context types with the types array.
	// The types slice is protected with mutex as the slice can be expanded while
	// worker threads are scanning earlier batches.
	mu    sync.RWMutex
	types []super.Type
}

var _ super.TypeFetcher = (*Decoder)(nil)

func NewDecoder(sctx *super.Context) *Decoder {
	return &Decoder{sctx: sctx}
}

func (d *Decoder) decode(b *buffer) error {
	for b.length() > 0 {
		code, err := b.ReadByte()
		if err != nil {
			return err
		}
		switch code {
		case TypeDefRecord:
			err = d.readTypeRecord(b)
		case TypeDefSet:
			err = d.readTypeSet(b)
		case TypeDefArray:
			err = d.readTypeArray(b)
		case TypeDefMap:
			err = d.readTypeMap(b)
		case TypeDefUnion:
			err = d.readTypeUnion(b)
		case TypeDefEnum:
			err = d.readTypeEnum(b)
		case TypeDefName:
			err = d.readTypeName(b)
		case TypeDefError:
			err = d.readTypeError(b)
		default:
			return fmt.Errorf("unknown BSON typedef code: %d", code)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *Decoder) readTypeRecord(b *buffer) error {
	nfields, err := readUvarintAsInt(b)
	if err != nil {
		return errBadFormat
	}
	var fields []super.Field
	for range nfields {
		f, err := d.readField(b)
		if err != nil {
			return err
		}
		fields = append(fields, f)
	}
	typ, err := d.sctx.LookupTypeRecord(fields)
	if err != nil {
		return err
	}
	d.enter(typ)
	return nil
}

func (d *Decoder) readField(b *buffer) (super.Field, error) {
	name, err := d.readCountedString(b)
	if err != nil {
		return super.Field{}, err
	}
	typ, err := d.readType(b)
	if err != nil {
		return super.Field{}, err
	}
	return super.NewField(name, typ), nil
}

func (d *Decoder) readTypeArray(b *buffer) error {
	inner, err := d.readType(b)
	if err != nil {
		return err
	}
	d.enter(d.sctx.LookupTypeArray(inner))
	return nil
}

func (d *Decoder) readTypeSet(b *buffer) error {
	inner, err := d.readType(b)
	if err != nil {
		return err
	}
	d.enter(d.sctx.LookupTypeSet(inner))
	return nil
}

func (d *Decoder) readTypeMap(b *buffer) error {
	keyType, err := d.readType(b)
	if err != nil {
		return err
	}
	valType, err := d.readType(b)
	if err != nil {
		return err
	}
	d.enter(d.sctx.LookupTypeMap(keyType, valType))
	return err
}

func (d *Decoder) readTypeUnion(b *buffer) error {
	ntyp, err := readUvarintAsInt(b)
	if err != nil {
		return errBadFormat
	}
	if ntyp == 0 {
		return errors.New("type union: zero types not allowed")
	}
	if ntyp > super.MaxUnionTypes {
		return fmt.Errorf("type union: too many types (%d)", ntyp)

	}
	types := make([]super.Type, 0, ntyp)
	for range ntyp {
		typ, err := d.readType(b)
		if err != nil {
			return err
		}
		types = append(types, typ)
	}
	d.enter(d.sctx.LookupTypeUnion(types))
	return nil
}

func (d *Decoder) readTypeEnum(b *buffer) error {
	nsym, err := readUvarintAsInt(b)
	if err != nil {
		return errBadFormat
	}
	if nsym > super.MaxEnumSymbols {
		return fmt.Errorf("too many enum symbols encountered (%d)", nsym)
	}
	var symbols []string
	for range nsym {
		s, err := d.readCountedString(b)
		if err != nil {
			return err
		}
		symbols = append(symbols, s)
	}
	d.enter(d.sctx.LookupTypeEnum(symbols))
	return nil
}

func (d *Decoder) readCountedString(b *buffer) (string, error) {
	n, err := readUvarintAsInt(b)
	if err != nil {
		return "", errBadFormat
	}
	name, err := b.read(n)
	if err != nil {
		return "", errBadFormat
	}
	// pull the name out before the next read which might overwrite the buffer
	return string(name), nil
}

func (d *Decoder) readTypeName(b *buffer) error {
	name, err := d.readCountedString(b)
	if err != nil {
		return err
	}
	inner, err := d.readType(b)
	if err != nil {
		return err
	}
	typ, err := d.sctx.LookupTypeNamed(name, inner)
	if err != nil {
		return err
	}
	d.enter(typ)
	return nil
}

func (d *Decoder) readTypeError(b *buffer) error {
	inner, err := d.readType(b)
	if err != nil {
		return err
	}
	d.enter(d.sctx.LookupTypeError(inner))
	return nil
}

func (d *Decoder) readType(b *buffer) (super.Type, error) {
	id, err := readUvarintAsInt(b)
	if err != nil {
		return nil, errBadFormat
	}
	return d.LookupType(id)
}

func (d *Decoder) LookupType(id int) (super.Type, error) {
	if id < super.IDTypeComplex {
		return super.LookupPrimitiveByID(id)
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	off := id - super.IDTypeComplex
	if off < len(d.types) {
		return d.types[off], nil
	}
	return nil, fmt.Errorf("no type found for type id %d", id)
}

func (d *Decoder) enter(typ super.Type) {
	// Even though type decoding is single threaded, workers processing a
	// previous batch can be accessing the types map (via LookupType) while
	// the single thread is extending it so these accesses are protected
	// with the mutex.
	d.mu.Lock()
	d.types = append(d.types, typ)
	d.mu.Unlock()
}
