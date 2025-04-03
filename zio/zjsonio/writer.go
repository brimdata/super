package zjsonio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zcode"
)

type Object struct {
	Type  zType       `json:"type"`
	Value interface{} `json:"value"`
}

func unmarshal(b []byte) (*Object, error) {
	var object Object
	if err := unpacker.Unmarshal(b, &object); err != nil {
		return nil, fmt.Errorf("malformed ZJSON: bad type object: %q: %w", bytes.TrimSpace(b), err)
	}
	return &object, nil
}

type Writer struct {
	writer  io.WriteCloser
	zctx    *super.Context
	types   map[super.Type]super.Type
	encoder encoder
}

func NewWriter(w io.WriteCloser) *Writer {
	return &Writer{
		writer:  w,
		zctx:    super.NewContext(),
		types:   make(map[super.Type]super.Type),
		encoder: make(encoder),
	}
}

func (w *Writer) Close() error {
	return w.writer.Close()
}

func (w *Writer) Write(val super.Value) error {
	rec, err := w.Transform(&val)
	if err != nil {
		return err
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	_, err = w.writer.Write(b)
	if err != nil {
		return err
	}
	return w.write("\n")
}

func (w *Writer) write(s string) error {
	_, err := w.writer.Write([]byte(s))
	return err
}

func (w *Writer) Transform(r *super.Value) (Object, error) {
	local, ok := w.types[r.Type()]
	if !ok {
		var err error
		local, err = w.zctx.TranslateType(r.Type())
		if err != nil {
			return Object{}, err
		}
		w.types[r.Type()] = local
	}
	// Encode type before encoding value in case there are type values
	// in the value.  We want to keep the order consistent.
	typ := w.encoder.encodeType(local)
	v, err := w.encodeValue(w.zctx, local, r.Bytes())
	if err != nil {
		return Object{}, err
	}
	return Object{
		Type:  typ,
		Value: v,
	}, nil
}

func (w *Writer) encodeValue(zctx *super.Context, typ super.Type, val zcode.Bytes) (interface{}, error) {
	if val == nil {
		return nil, nil
	}
	switch typ := typ.(type) {
	case *super.TypeRecord:
		return w.encodeRecord(zctx, typ, val)
	case *super.TypeArray:
		return w.encodeContainer(zctx, typ.Type, val)
	case *super.TypeSet:
		return w.encodeContainer(zctx, typ.Type, val)
	case *super.TypeMap:
		return w.encodeMap(zctx, typ, val)
	case *super.TypeUnion:
		return w.encodeUnion(zctx, typ, val)
	case *super.TypeEnum:
		return w.encodePrimitive(zctx, super.TypeUint64, val)
	case *super.TypeError:
		return w.encodeValue(zctx, typ.Type, val)
	case *super.TypeNamed:
		return w.encodeValue(zctx, typ.Type, val)
	case *super.TypeOfType:
		inner, err := w.zctx.LookupByValue(val)
		if err != nil {
			return nil, err
		}
		return w.encoder.encodeType(inner), nil
	default:
		return w.encodePrimitive(zctx, typ, val)
	}
}

func (w *Writer) encodeRecord(zctx *super.Context, typ *super.TypeRecord, val zcode.Bytes) (interface{}, error) {
	// We start out with a slice that contains nothing instead of nil
	// so that an empty container encodes as a JSON empty array [].
	out := []interface{}{}
	for k, it := 0, val.Iter(); !it.Done(); k++ {
		v, err := w.encodeValue(zctx, typ.Fields[k].Type, it.Next())
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (w *Writer) encodeContainer(zctx *super.Context, typ super.Type, bytes zcode.Bytes) (interface{}, error) {
	// We start out with a slice that contains nothing instead of nil
	// so that an empty container encodes as a JSON empty array [].
	out := []interface{}{}
	for it := bytes.Iter(); !it.Done(); {
		v, err := w.encodeValue(zctx, typ, it.Next())
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (w *Writer) encodeMap(zctx *super.Context, typ *super.TypeMap, v zcode.Bytes) (interface{}, error) {
	// We start out with a slice that contains nothing instead of nil
	// so that an empty map encodes as a JSON empty array [].
	out := []interface{}{}
	for it := v.Iter(); !it.Done(); {
		pair := make([]interface{}, 2)
		var err error
		pair[0], err = w.encodeValue(zctx, typ.KeyType, it.Next())
		if err != nil {
			return nil, err
		}
		pair[1], err = w.encodeValue(zctx, typ.ValType, it.Next())
		if err != nil {
			return nil, err
		}
		out = append(out, pair)
	}
	return out, nil
}

func (w *Writer) encodeUnion(zctx *super.Context, union *super.TypeUnion, bytes zcode.Bytes) (interface{}, error) {
	inner, b := union.Untag(bytes)
	val, err := w.encodeValue(zctx, inner, b)
	if err != nil {
		return nil, err
	}
	return []interface{}{strconv.Itoa(union.TagOf(inner)), val}, nil
}

func (w *Writer) encodePrimitive(zctx *super.Context, typ super.Type, v zcode.Bytes) (interface{}, error) {
	if typ == super.TypeType {
		typ, err := zctx.LookupByValue(v)
		if err != nil {
			return nil, err
		}
		if super.TypeID(typ) < super.IDTypeComplex {
			return super.PrimitiveName(typ), nil
		}
		if named, ok := typ.(*super.TypeNamed); ok {
			return named.Name, nil
		}
		return strconv.Itoa(super.TypeID(typ)), nil
	}
	if typ.ID() == super.IDString {
		return string(v), nil
	}
	return sup.FormatPrimitive(typ, v), nil
}
