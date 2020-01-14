package ndjsonio

import (
	"bytes"
	"errors"
	"fmt"
	"sort"

	"github.com/buger/jsonparser"
	"github.com/mccanne/zq/zcode"
	"github.com/mccanne/zq/zio/zeekio"
	"github.com/mccanne/zq/zng"
)

// ErrMultiTypedVector signifies that a json array was found with multiple types.
// Multiple-typed arrays are unsupported at this time. See zq#64.
var ErrMultiTypedVector = errors.New("vectors with multiple types are not supported")

type Parser struct {
	builder *zcode.Builder
	scratch []byte
}

func NewParser() *Parser {
	return &Parser{builder: zcode.NewBuilder()}
}

// Parse returns a zng.Encoding slice as well as an inferred zng.Type
// from the provided JSON input. The function expects the input json to be an
// object, otherwise an error is returned.
func (p *Parser) Parse(b []byte) (zcode.Bytes, zng.Type, error) {
	val, typ, _, err := jsonparser.Get(b)
	if err != nil {
		return nil, nil, err
	}
	if typ != jsonparser.Object {
		return nil, nil, fmt.Errorf("expected JSON type to be Object but got %#v", typ)
	}
	p.builder.Reset()
	ztyp, err := p.jsonParseObject(val)
	if err != nil {
		return nil, nil, err
	}
	return p.builder.Encode(), ztyp, nil
}

type stubTypeOf struct{}

var stubType = &stubTypeOf{}

func (t *stubTypeOf) String() string {
	return "none"
}

func (t *stubTypeOf) Parse(in []byte) (zcode.Bytes, error) {
	return nil, nil
}

func (t *stubTypeOf) Format(value []byte) (interface{}, error) {
	return "none", nil
}

func (t *stubTypeOf) StringOf(zv zcode.Bytes) string {
	return "-"
}

func (t *stubTypeOf) Marshal(zv zcode.Bytes) (interface{}, error) {
	return nil, nil
}

func (t *stubTypeOf) Coerce(zv zcode.Bytes, typ zng.Type) zcode.Bytes {
	return nil
}

func (p *Parser) jsonParseObject(b []byte) (zng.Type, error) {
	type kv struct {
		key   []byte
		value []byte
		typ   jsonparser.ValueType
	}
	var kvs []kv
	err := jsonparser.ObjectEach(b, func(key []byte, value []byte, typ jsonparser.ValueType, offset int) error {
		kvs = append(kvs, kv{key, value, typ})
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Sort fields lexigraphically ensuring maps with the same
	// columns but different printed order get assigned the same descriptor.
	sort.Slice(kvs, func(i, j int) bool {
		return bytes.Compare(kvs[i].key, kvs[j].key) < 0
	})

	// Build the list of columns (without types yet) and then run them
	// through Unflatten() to find nested records.
	columns := make([]zng.Column, len(kvs))
	for i, kv := range kvs {
		columns[i] = zng.Column{Name: string(kv.key), Type: stubType}
	}
	columns, _ = zeekio.Unflatten(columns, false)

	// Parse the actual values and fill in column types along the way,
	// taking care to step into nested records as necessary.
	colno := 0
	nestedColno := 0
	for _, kv := range kvs {
		recType, isRecord := columns[colno].Type.(*zng.TypeRecord)
		if isRecord {
			if nestedColno == 0 {
				p.builder.BeginContainer()
			}
		}

		ztyp, err := p.jsonParseValue(kv.value, kv.typ)
		if err != nil {
			return nil, err
		}

		if isRecord {
			recType.Columns[nestedColno].Type = ztyp
			nestedColno += 1
			if nestedColno == len(recType.Columns) {
				p.builder.EndContainer()
				nestedColno = 0
				colno += 1
			}
		} else {
			columns[colno].Type = ztyp
			colno += 1
		}
	}
	return &zng.TypeRecord{Columns: columns}, nil
}

func (p *Parser) jsonParseValue(raw []byte, typ jsonparser.ValueType) (zng.Type, error) {
	switch typ {
	case jsonparser.Array:
		p.builder.BeginContainer()
		defer p.builder.EndContainer()
		return p.jsonParseArray(raw)
	case jsonparser.Object:
		p.builder.BeginContainer()
		defer p.builder.EndContainer()
		return p.jsonParseObject(raw)
	case jsonparser.Boolean:
		return p.jsonParseBool(raw)
	case jsonparser.Number:
		return p.jsonParseNumber(raw)
	case jsonparser.Null:
		return p.jsonParseString(nil)
	case jsonparser.String:
		return p.jsonParseString(raw)
	default:
		return nil, fmt.Errorf("unsupported type %v", typ)
	}
}

func (p *Parser) jsonParseArray(raw []byte) (zng.Type, error) {
	var err error
	var types []zng.Type
	jsonparser.ArrayEach(raw, func(el []byte, typ jsonparser.ValueType, offset int, elErr error) {
		if err != nil {
			return
		}
		if elErr != nil {
			err = elErr
		}
		var ztyp zng.Type
		ztyp, err = p.jsonParseValue(el, typ)
		types = append(types, ztyp)
	})
	if err != nil {
		return nil, err
	}
	if len(types) == 0 {
		return zng.LookupVectorType(zng.TypeString), nil
	}
	var vType zng.Type
	for _, t := range types {
		if vType == nil {
			vType = t
		} else if vType != t {
			return nil, ErrMultiTypedVector
		}
	}
	return zng.LookupVectorType(vType), nil
}

func (p *Parser) jsonParseBool(b []byte) (zng.Type, error) {
	boolean, err := jsonparser.GetBoolean(b)
	if err != nil {
		return nil, err
	}
	p.builder.Append(zng.EncodeBool(boolean), false)
	return zng.TypeBool, nil
}

func (p *Parser) jsonParseNumber(b []byte) (zng.Type, error) {
	d, err := zng.UnsafeParseFloat64(b)
	if err != nil {
		return nil, err
	}
	p.builder.Append(zng.EncodeDouble(d), false)
	return zng.TypeDouble, nil
}

func (p *Parser) jsonParseString(b []byte) (zng.Type, error) {
	p.builder.Append(zng.Unescape(b), false)
	return zng.TypeString, nil
}
