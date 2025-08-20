package jsupio

import (
	"errors"
	"fmt"

	"github.com/brimdata/super"
)

type zType interface {
	typeNode()
}

type (
	zPrimitive struct {
		Kind string `json:"kind" unpack:"primitive"`
		Name string `json:"name"`
	}
	zRecord struct {
		Kind   string   `json:"kind" unpack:"record"`
		ID     int      `json:"id"`
		Fields []zField `json:"fields"`
	}
	zField struct {
		Name string `json:"name"`
		Type zType  `json:"type"`
	}
	zArray struct {
		Kind string `json:"kind" unpack:"array"`
		ID   int    `json:"id"`
		Type zType  `json:"type"`
	}
	zSet struct {
		Kind string `json:"kind" unpack:"set"`
		ID   int    `json:"id"`
		Type zType  `json:"type"`
	}
	zMap struct {
		Kind    string `json:"kind" unpack:"map"`
		ID      int    `json:"id"`
		KeyType zType  `json:"key_type"`
		ValType zType  `json:"val_type"`
	}
	zUnion struct {
		Kind  string  `json:"kind" unpack:"union"`
		ID    int     `json:"id"`
		Types []zType `json:"types"`
	}
	zEnum struct {
		Kind    string   `json:"kind" unpack:"enum"`
		ID      int      `json:"id"`
		Symbols []string `json:"symbols"`
	}
	zError struct {
		Kind string `json:"kind" unpack:"error"`
		ID   int    `json:"id"`
		Type zType  `json:"type"`
	}
	zNamed struct {
		Kind string `json:"kind" unpack:"named"`
		ID   int    `json:"id"`
		Name string `json:"name"`
		Type zType  `json:"type"`
	}
	zRef struct {
		Kind string `json:"kind" unpack:"ref"`
		ID   int    `json:"id"`
	}
)

func (*zPrimitive) typeNode() {}
func (*zRecord) typeNode()    {}
func (*zArray) typeNode()     {}
func (*zSet) typeNode()       {}
func (*zMap) typeNode()       {}
func (*zUnion) typeNode()     {}
func (*zEnum) typeNode()      {}
func (*zError) typeNode()     {}
func (*zNamed) typeNode()     {}
func (*zRef) typeNode()       {}

type encoder map[super.Type]zType

func (e encoder) encodeType(typ super.Type) zType {
	t, ok := e[typ]
	if !ok {
		t = e.newType(typ)
		id := super.TypeID(typ)
		if id < super.IDTypeComplex {
			e[typ] = t
		} else {
			e[typ] = &zRef{
				Kind: "ref",
				ID:   id,
			}
		}
	}
	return t
}

func (e encoder) newType(typ super.Type) zType {
	switch typ := typ.(type) {
	case *super.TypeNamed:
		t := e.encodeType(typ.Type)
		return &zNamed{
			Kind: "named",
			ID:   super.TypeID(typ),
			Name: typ.Name,
			Type: t,
		}
	case *super.TypeRecord:
		var fields []zField
		for _, f := range typ.Fields {
			fields = append(fields, zField{
				Name: f.Name,
				Type: e.encodeType(f.Type),
			})
		}
		return &zRecord{
			Kind:   "record",
			ID:     super.TypeID(typ),
			Fields: fields,
		}
	case *super.TypeArray:
		return &zArray{
			Kind: "array",
			ID:   super.TypeID(typ),
			Type: e.encodeType(typ.Type),
		}
	case *super.TypeSet:
		return &zSet{
			Kind: "set",
			ID:   super.TypeID(typ),
			Type: e.encodeType(typ.Type),
		}
	case *super.TypeUnion:
		var types []zType
		for _, typ := range typ.Types {
			types = append(types, e.encodeType(typ))
		}
		return &zUnion{
			Kind:  "union",
			ID:    super.TypeID(typ),
			Types: types,
		}
	case *super.TypeEnum:
		return &zEnum{
			Kind:    "enum",
			ID:      super.TypeID(typ),
			Symbols: typ.Symbols,
		}
	case *super.TypeMap:
		return &zMap{
			Kind:    "map",
			ID:      super.TypeID(typ),
			KeyType: e.encodeType(typ.KeyType),
			ValType: e.encodeType(typ.ValType),
		}
	case *super.TypeError:
		return &zError{
			Kind: "error",
			ID:   super.TypeID(typ),
			Type: e.encodeType(typ.Type),
		}
	default:
		return &zPrimitive{
			Kind: "primitive",
			Name: super.PrimitiveName(typ),
		}
	}
}

type decoder map[int]super.Type

func (d decoder) decodeType(sctx *super.Context, t zType) (super.Type, error) {
	switch t := t.(type) {
	case *zRecord:
		typ, err := d.decodeTypeRecord(sctx, t)
		d[t.ID] = typ
		return typ, err
	case *zArray:
		inner, err := d.decodeType(sctx, t.Type)
		if err != nil {
			return nil, err
		}
		typ := sctx.LookupTypeArray(inner)
		d[t.ID] = typ
		return typ, nil
	case *zSet:
		inner, err := d.decodeType(sctx, t.Type)
		if err != nil {
			return nil, err
		}
		typ := sctx.LookupTypeSet(inner)
		d[t.ID] = typ
		return typ, nil
	case *zUnion:
		typ, err := d.decodeTypeUnion(sctx, t)
		d[t.ID] = typ
		return typ, err
	case *zEnum:
		typ, err := d.decodeTypeEnum(sctx, t)
		d[t.ID] = typ
		return typ, err
	case *zMap:
		typ, err := d.decodeTypeMap(sctx, t)
		d[t.ID] = typ
		return typ, err
	case *zNamed:
		inner, err := d.decodeType(sctx, t.Type)
		if err != nil {
			return nil, err
		}
		typ, err := sctx.LookupTypeNamed(t.Name, inner)
		d[t.ID] = typ
		return typ, err
	case *zError:
		inner, err := d.decodeType(sctx, t.Type)
		if err != nil {
			return nil, err
		}
		typ := sctx.LookupTypeError(inner)
		d[t.ID] = typ
		return typ, nil
	case *zPrimitive:
		typ := super.LookupPrimitive(t.Name)
		if typ == nil {
			return nil, errors.New("JSUP unknown type: " + t.Name)
		}
		return typ, nil
	case *zRef:
		typ, ok := d[t.ID]
		if !ok {
			return nil, fmt.Errorf("JSUP unknown type reference: %d", t.ID)
		}
		return typ, nil
	}
	return nil, fmt.Errorf("JSUP unknown type: %T", t)
}

func (d decoder) decodeTypeRecord(sctx *super.Context, typ *zRecord) (*super.TypeRecord, error) {
	fields := make([]super.Field, 0, len(typ.Fields))
	for _, field := range typ.Fields {
		typ, err := d.decodeType(sctx, field.Type)
		if err != nil {
			return nil, err
		}
		fields = append(fields, super.NewField(field.Name, typ))
	}
	return sctx.LookupTypeRecord(fields)
}

func (d decoder) decodeTypeUnion(sctx *super.Context, union *zUnion) (*super.TypeUnion, error) {
	var types []super.Type
	for _, t := range union.Types {
		typ, err := d.decodeType(sctx, t)
		if err != nil {
			return nil, err
		}
		types = append(types, typ)
	}
	return sctx.LookupTypeUnion(types), nil
}

func (d decoder) decodeTypeMap(sctx *super.Context, m *zMap) (*super.TypeMap, error) {
	keyType, err := d.decodeType(sctx, m.KeyType)
	if err != nil {
		return nil, err
	}
	valType, err := d.decodeType(sctx, m.ValType)
	if err != nil {
		return nil, err
	}
	return sctx.LookupTypeMap(keyType, valType), nil
}

func (d decoder) decodeTypeEnum(sctx *super.Context, enum *zEnum) (*super.TypeEnum, error) {
	return sctx.LookupTypeEnum(enum.Symbols), nil
}
