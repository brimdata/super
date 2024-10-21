package zngio

import (
	"encoding/binary"
	"errors"
	"fmt"

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
	zctx    *super.Context
	encoded map[super.Type]super.Type
	bytes   []byte
}

func NewEncoder() *Encoder {
	return &Encoder{
		zctx:    super.NewContext(),
		encoded: make(map[super.Type]super.Type),
	}
}

func (e *Encoder) Reset() {
	e.bytes = e.bytes[:0]
	e.encoded = make(map[super.Type]super.Type)
	e.zctx.Reset()
}

func (e *Encoder) Flush() {
	e.bytes = e.bytes[:0]
}

func (e *Encoder) Lookup(external super.Type) super.Type {
	return e.encoded[external]
}

// Encode takes a type from outside this context and constructs a type from
// inside this context and emits ZNG typedefs for any type needed to construct
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
	typ, err := e.zctx.LookupTypeRecord(fields)
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
	typ := e.zctx.LookupTypeUnion(types)
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
	typ := e.zctx.LookupTypeSet(inner)
	e.bytes = append(e.bytes, TypeDefSet)
	e.bytes = binary.AppendUvarint(e.bytes, uint64(super.TypeID(inner)))
	return typ, nil
}

func (e *Encoder) encodeTypeArray(ext *super.TypeArray) (*super.TypeArray, error) {
	inner, err := e.Encode(ext.Type)
	if err != nil {
		return nil, err
	}
	typ := e.zctx.LookupTypeArray(inner)
	e.bytes = append(e.bytes, TypeDefArray)
	e.bytes = binary.AppendUvarint(e.bytes, uint64(super.TypeID(inner)))
	return typ, nil
}

func (e *Encoder) encodeTypeEnum(ext *super.TypeEnum) (*super.TypeEnum, error) {
	symbols := ext.Symbols
	typ := e.zctx.LookupTypeEnum(symbols)
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
	typ := e.zctx.LookupTypeMap(keyType, valType)
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
	typ, err := e.zctx.LookupTypeNamed(ext.Name, inner)
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
	typ := e.zctx.LookupTypeError(inner)
	e.bytes = append(e.bytes, TypeDefError)
	e.bytes = binary.AppendUvarint(e.bytes, uint64(super.TypeID(typ.Type)))
	return typ, nil
}

type localctx struct {
	// internal context implied by ZNG file
	zctx *super.Context
	// mapper to map internal to shared type contexts
	mapper *super.Mapper
}

// Called at end-of-stream... XXX elaborate
func (l *localctx) reset(shared *super.Context) {
	l.zctx = super.NewContext()
	l.mapper = super.NewMapper(shared)
}

type Decoder struct {
	// shared/output context
	zctx *super.Context
	// local context and mapper from local to shared
	local localctx
}

func NewDecoder(zctx *super.Context) *Decoder {
	d := &Decoder{zctx: zctx}
	d.reset()
	return d
}

func (d *Decoder) reset() {
	d.local.reset(d.zctx)
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
			return fmt.Errorf("unknown ZNG typedef code: %d", code)
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
	for k := 0; k < nfields; k++ {
		f, err := d.readField(b)
		if err != nil {
			return err
		}
		fields = append(fields, f)
	}
	typ, err := d.local.zctx.LookupTypeRecord(fields)
	if err != nil {
		return err
	}
	_, err = d.local.mapper.Enter(typ)
	return err
}

func (d *Decoder) readField(b *buffer) (super.Field, error) {
	name, err := d.readCountedString(b)
	if err != nil {
		return super.Field{}, err
	}
	id, err := readUvarintAsInt(b)
	if err != nil {
		return super.Field{}, errBadFormat
	}
	typ, err := d.local.zctx.LookupType(id)
	if err != nil {
		return super.Field{}, err
	}
	return super.NewField(name, typ), nil
}

func (d *Decoder) readTypeArray(b *buffer) error {
	id, err := readUvarintAsInt(b)
	if err != nil {
		return errBadFormat
	}
	inner, err := d.local.zctx.LookupType(id)
	if err != nil {
		return err
	}
	typ := d.local.zctx.LookupTypeArray(inner)
	_, err = d.local.mapper.Enter(typ)
	return err
}

func (d *Decoder) readTypeSet(b *buffer) error {
	id, err := readUvarintAsInt(b)
	if err != nil {
		return errBadFormat
	}
	innerType, err := d.local.zctx.LookupType(id)
	if err != nil {
		return err
	}
	typ := d.local.zctx.LookupTypeSet(innerType)
	_, err = d.local.mapper.Enter(typ)
	return err
}

func (d *Decoder) readTypeMap(b *buffer) error {
	id, err := readUvarintAsInt(b)
	if err != nil {
		return errBadFormat
	}
	keyType, err := d.local.zctx.LookupType(id)
	if err != nil {
		return err
	}
	id, err = readUvarintAsInt(b)
	if err != nil {
		return errBadFormat
	}
	valType, err := d.local.zctx.LookupType(id)
	if err != nil {
		return err
	}
	typ := d.local.zctx.LookupTypeMap(keyType, valType)
	_, err = d.local.mapper.Enter(typ)
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
	var types []super.Type
	for k := 0; k < ntyp; k++ {
		id, err := readUvarintAsInt(b)
		if err != nil {
			return errBadFormat
		}
		typ, err := d.local.zctx.LookupType(id)
		if err != nil {
			return err
		}
		types = append(types, typ)
	}
	typ := d.local.zctx.LookupTypeUnion(types)
	_, err = d.local.mapper.Enter(typ)
	return err
}

func (d *Decoder) readTypeEnum(b *buffer) error {
	nsym, err := readUvarintAsInt(b)
	if err != nil {
		return errBadFormat
	}
	var symbols []string
	for k := 0; k < nsym; k++ {
		s, err := d.readCountedString(b)
		if err != nil {
			return err
		}
		symbols = append(symbols, s)
	}
	typ := d.local.zctx.LookupTypeEnum(symbols)
	_, err = d.local.mapper.Enter(typ)
	return err
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
	id, err := readUvarintAsInt(b)
	if err != nil {
		return errBadFormat
	}
	inner, err := d.local.zctx.LookupType(id)
	if err != nil {
		return err
	}
	typ, err := d.local.zctx.LookupTypeNamed(name, inner)
	if err != nil {
		return err
	}
	_, err = d.local.mapper.Enter(typ)
	return err
}

func (d *Decoder) readTypeError(b *buffer) error {
	id, err := readUvarintAsInt(b)
	if err != nil {
		return errBadFormat
	}
	inner, err := d.local.zctx.LookupType(id)
	if err != nil {
		return err
	}
	typ := d.local.zctx.LookupTypeError(inner)
	_, err = d.local.mapper.Enter(typ)
	return err
}
