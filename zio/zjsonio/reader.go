package zjsonio

import (
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/skim"
	"github.com/brimdata/super/zcode"
	"github.com/brimdata/super/zson"
)

const (
	ReadSize    = 64 * 1024
	MaxLineSize = 50 * 1024 * 1024
)

type Reader struct {
	scanner *skim.Scanner
	zctx    *super.Context
	decoder decoder
	builder *zcode.Builder
	val     super.Value
}

func NewReader(zctx *super.Context, reader io.Reader) *Reader {
	buffer := make([]byte, ReadSize)
	return &Reader{
		scanner: skim.NewScanner(reader, buffer, MaxLineSize),
		zctx:    zctx,
		decoder: make(decoder),
		builder: zcode.NewBuilder(),
	}
}

func (r *Reader) Read() (*super.Value, error) {
	e := func(err error) error {
		if err == nil {
			return err
		}
		return fmt.Errorf("line %d: %w", r.scanner.Stats.Lines, err)
	}

	line, err := r.scanner.ScanLine()
	if line == nil {
		return nil, e(err)
	}
	object, err := unmarshal(line)
	if err != nil {
		return nil, e(err)
	}
	typ, err := r.decoder.decodeType(r.zctx, object.Type)
	if err != nil {
		return nil, err
	}
	r.builder.Truncate()
	if err := r.decodeValue(r.builder, typ, object.Value); err != nil {
		return nil, e(err)
	}
	r.val = super.NewValue(typ, r.builder.Bytes().Body())
	return &r.val, nil
}

func (r *Reader) decodeValue(b *zcode.Builder, typ super.Type, body interface{}) error {
	if body == nil {
		b.Append(nil)
		return nil
	}
	switch typ := typ.(type) {
	case *super.TypeNamed:
		return r.decodeValue(b, typ.Type, body)
	case *super.TypeUnion:
		return r.decodeUnion(b, typ, body)
	case *super.TypeMap:
		return r.decodeMap(b, typ, body)
	case *super.TypeEnum:
		return r.decodeEnum(b, typ, body)
	case *super.TypeRecord:
		return r.decodeRecord(b, typ, body)
	case *super.TypeArray:
		return r.decodeContainer(b, typ.Type, body, "array")
	case *super.TypeSet:
		b.BeginContainer()
		err := r.decodeContainerBody(b, typ.Type, body, "set")
		b.TransformContainer(super.NormalizeSet)
		b.EndContainer()
		return err
	case *super.TypeError:
		return r.decodeValue(b, typ.Type, body)
	case *super.TypeOfType:
		var t zType
		if err := unpacker.UnmarshalObject(body, &t); err != nil {
			return fmt.Errorf("type value is not a valid ZJSON type: %w", err)
		}
		local, err := r.decoder.decodeType(r.zctx, t)
		if err != nil {
			return err
		}
		tv := r.zctx.LookupTypeValue(local)
		b.Append(tv.Bytes())
		return nil
	default:
		return r.decodePrimitive(b, typ, body)
	}
}

func (r *Reader) decodeRecord(b *zcode.Builder, typ *super.TypeRecord, v interface{}) error {
	values, ok := v.([]interface{})
	if !ok {
		return errors.New("ZJSON record value must be a JSON array")
	}
	fields := typ.Fields
	b.BeginContainer()
	for k, val := range values {
		if k >= len(fields) {
			return errors.New("record with extra field")

		}
		// Each field is either a string value or an array of string values.
		if err := r.decodeValue(b, fields[k].Type, val); err != nil {
			return err
		}
	}
	b.EndContainer()
	return nil
}

func (r *Reader) decodePrimitive(builder *zcode.Builder, typ super.Type, v interface{}) error {
	if super.IsContainerType(typ) && !super.IsUnionType(typ) {
		return errors.New("expected primitive type, got container")
	}
	text, ok := v.(string)
	if !ok {
		return errors.New("ZJSON primitive value is not a JSON string")
	}
	return zson.BuildPrimitive(builder, zson.Primitive{
		Type: typ,
		Text: text,
	})
}

func (r *Reader) decodeContainerBody(b *zcode.Builder, typ super.Type, body interface{}, which string) error {
	items, ok := body.([]interface{})
	if !ok {
		return fmt.Errorf("bad JSON for ZJSON %s value", which)
	}
	for _, item := range items {
		if err := r.decodeValue(b, typ, item); err != nil {
			return err
		}
	}
	return nil
}

func (r *Reader) decodeContainer(b *zcode.Builder, typ super.Type, body interface{}, which string) error {
	b.BeginContainer()
	err := r.decodeContainerBody(b, typ, body, which)
	b.EndContainer()
	return err
}

func (r *Reader) decodeUnion(builder *zcode.Builder, typ *super.TypeUnion, body interface{}) error {
	tuple, ok := body.([]interface{})
	if !ok {
		return errors.New("bad JSON for ZJSON union value")
	}
	if len(tuple) != 2 {
		return errors.New("ZJSON union value not an array of two elements")
	}
	tagStr, ok := tuple[0].(string)
	if !ok {
		return errors.New("bad tag for ZJSON union value")
	}
	tag, err := strconv.Atoi(tagStr)
	if err != nil {
		return fmt.Errorf("bad tag for ZJSON union value: %w", err)
	}
	inner, err := typ.Type(tag)
	if err != nil {
		return fmt.Errorf("bad tag for ZJSON union value: %w", err)
	}
	builder.BeginContainer()
	builder.Append(super.EncodeInt(int64(tag)))
	if err := r.decodeValue(builder, inner, tuple[1]); err != nil {
		return err
	}
	builder.EndContainer()
	return nil
}

func (r *Reader) decodeMap(b *zcode.Builder, typ *super.TypeMap, body interface{}) error {
	items, ok := body.([]interface{})
	if !ok {
		return errors.New("bad JSON for ZJSON union value")
	}
	b.BeginContainer()
	for _, item := range items {
		pair, ok := item.([]interface{})
		if !ok || len(pair) != 2 {
			return errors.New("ZJSON map value must be an array of two-element arrays")
		}
		if err := r.decodeValue(b, typ.KeyType, pair[0]); err != nil {
			return err
		}
		if err := r.decodeValue(b, typ.ValType, pair[1]); err != nil {
			return err
		}
	}
	b.EndContainer()
	return nil
}

func (r *Reader) decodeEnum(b *zcode.Builder, typ *super.TypeEnum, body interface{}) error {
	s, ok := body.(string)
	if !ok {
		return errors.New("ZJSON enum index value is not a JSON string")
	}
	index, err := strconv.Atoi(s)
	if err != nil {
		return errors.New("ZJSON enum index value is not a string integer")
	}
	b.Append(super.EncodeUint(uint64(index)))
	return nil
}
