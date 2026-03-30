package bsupio

import (
	"errors"
	"fmt"
	"sync"

	"github.com/brimdata/super"
)

type Encoder struct {
	defs *super.TypeDefs
	off  int
}

func NewEncoder() *Encoder {
	return &Encoder{
		defs: super.NewTypeDefs(),
	}
}

func (e *Encoder) Reset() {
	e.defs.Reset()
	e.off = 0
}

func (e *Encoder) Flush() {
	e.off = e.defs.Len()
}

func (e *Encoder) Len() int {
	return e.defs.Len() - e.off
}

func (e *Encoder) nextBuffer() []byte {
	b := e.defs.Bytes()[e.off:]
	e.off = e.defs.Len()
	return b
}

// Encode takes a type from outside this context and constructs a type from
// inside this context and emits BSUP typedefs for any type needed to construct
// the new type into the buffer provided.
func (e *Encoder) Encode(external super.Type) uint32 {
	return e.defs.LookupType(external)
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
		case super.TypeDefRecord:
			err = d.readTypeRecord(b)
		case super.TypeDefSet:
			err = d.readTypeSet(b)
		case super.TypeDefArray:
			err = d.readTypeArray(b)
		case super.TypeDefMap:
			err = d.readTypeMap(b)
		case super.TypeDefUnion:
			err = d.readTypeUnion(b)
		case super.TypeDefEnum:
			err = d.readTypeEnum(b)
		case super.TypeDefNamed:
			err = d.readTypeName(b)
		case super.TypeDefError:
			err = d.readTypeError(b)
		case super.TypeDefFusion:
			err = d.readTypeFusion(b)
		default:
			return fmt.Errorf("unknown BSUP typedef code: %d", code)
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
	optByte, err := b.ReadByte()
	if err != nil {
		return super.Field{}, err
	}
	return super.NewFieldWithOpt(name, typ, optByte != 0), nil
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

func (d *Decoder) readTypeFusion(b *buffer) error {
	inner, err := d.readType(b)
	if err != nil {
		return err
	}
	d.enter(d.sctx.LookupTypeFusion(inner))
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
