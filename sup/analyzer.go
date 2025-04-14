package sup

import (
	"errors"
	"fmt"
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast"
)

type Value interface {
	TypeOf() super.Type
	SetType(super.Type)
}

// Note that all of the types include a generic super.Type as their type since
// anything can have a super.TypeNamed along with its normal type.
type (
	Primitive struct {
		Type super.Type
		Text string
	}
	Record struct {
		Type   super.Type
		Fields []Value
	}
	Array struct {
		Type     super.Type
		Elements []Value
	}
	Set struct {
		Type     super.Type
		Elements []Value
	}
	Union struct {
		Type  super.Type
		Tag   int
		Value Value
	}
	Enum struct {
		Type super.Type
		Name string
	}
	Map struct {
		Type    super.Type
		Entries []Entry
	}
	Entry struct {
		Key   Value
		Value Value
	}
	Null struct {
		Type super.Type
	}
	TypeValue struct {
		Type  super.Type
		Value super.Type
	}
	Error struct {
		Type  super.Type
		Value Value
	}
)

func (p *Primitive) TypeOf() super.Type { return p.Type }
func (r *Record) TypeOf() super.Type    { return r.Type }
func (a *Array) TypeOf() super.Type     { return a.Type }
func (s *Set) TypeOf() super.Type       { return s.Type }
func (u *Union) TypeOf() super.Type     { return u.Type }
func (e *Enum) TypeOf() super.Type      { return e.Type }
func (m *Map) TypeOf() super.Type       { return m.Type }
func (n *Null) TypeOf() super.Type      { return n.Type }
func (t *TypeValue) TypeOf() super.Type { return t.Type }
func (e *Error) TypeOf() super.Type     { return e.Type }

func (p *Primitive) SetType(t super.Type) { p.Type = t }
func (r *Record) SetType(t super.Type)    { r.Type = t }
func (a *Array) SetType(t super.Type)     { a.Type = t }
func (s *Set) SetType(t super.Type)       { s.Type = t }
func (u *Union) SetType(t super.Type)     { u.Type = t }
func (e *Enum) SetType(t super.Type)      { e.Type = t }
func (m *Map) SetType(t super.Type)       { m.Type = t }
func (n *Null) SetType(t super.Type)      { n.Type = t }
func (t *TypeValue) SetType(T super.Type) { t.Type = T }
func (e *Error) SetType(t super.Type)     { e.Type = t }

// An Analyzer transforms an ast.Value (which has decentralized type decorators)
// to a typed Value, where every component of a nested Value is explicitly typed.
// This is done via a semantic analysis where type state flows both down a the
// nested value hierarchy (via type decorators) and back up via fully typed value
// whose types are then usable as typedefs.  The Analyzer tracks the SUP typedef
// semantics by updating its table of name-to-type bindings in accordance with the
// left-to-right, depth-first semantics of SUP typedefs.
type Analyzer map[string]super.Type

func NewAnalyzer() Analyzer {
	return Analyzer(make(map[string]super.Type))
}

func (a Analyzer) ConvertValue(sctx *super.Context, val ast.Value) (Value, error) {
	return a.convertValue(sctx, val, nil)
}

func (a Analyzer) convertValue(sctx *super.Context, val ast.Value, parent super.Type) (Value, error) {
	switch val := val.(type) {
	case *ast.ImpliedValue:
		return a.convertAny(sctx, val.Of, parent)
	case *ast.DefValue:
		v, err := a.convertAny(sctx, val.Of, parent)
		if err != nil {
			return nil, err
		}
		named, err := a.enterTypeDef(sctx, val.TypeName, v.TypeOf())
		if err != nil {
			return nil, err
		}
		if named != nil {
			v.SetType(named)
		}
		return v, nil
	case *ast.CastValue:
		switch valOf := val.Of.(type) {
		case *ast.DefValue:
			// Enter the type def so val.Type can see it.
			if _, err := a.convertValue(sctx, valOf, nil); err != nil {
				return nil, err
			}
		case *ast.CastValue:
			// Enter any nested type defs so val.Type can see them.
			if _, err := a.convertType(sctx, valOf.Type); err != nil {
				return nil, err
			}
		}
		cast, err := a.convertType(sctx, val.Type)
		if err != nil {
			return nil, err
		}
		if err := a.typeCheck(cast, parent); err != nil {
			return nil, err
		}
		var v Value
		if union, ok := super.TypeUnder(cast).(*super.TypeUnion); ok {
			v, err = a.convertValue(sctx, val.Of, nil)
			if err != nil {
				return nil, err
			}
			v, err = a.convertUnion(sctx, v, union, cast)
		} else {
			v, err = a.convertValue(sctx, val.Of, cast)
		}
		if err != nil {
			return nil, err
		}
		if union, ok := super.TypeUnder(parent).(*super.TypeUnion); ok {
			v, err = a.convertUnion(sctx, v, union, parent)
		}
		return v, err
	}
	return nil, fmt.Errorf("unknown value ast type: %T", val)
}

func (a Analyzer) typeCheck(cast, parent super.Type) error {
	if parent == nil || cast == parent {
		return nil
	}
	if _, ok := super.TypeUnder(parent).(*super.TypeUnion); ok {
		// We let unions through this type check with no further checking
		// as any union incompability will be caught in convertAnyValue().
		return nil
	}
	return fmt.Errorf("decorator conflict enclosing context %q and decorator cast %q", FormatType(parent), FormatType(cast))
}

func (a Analyzer) enterTypeDef(sctx *super.Context, name string, typ super.Type) (*super.TypeNamed, error) {
	var named *super.TypeNamed
	if !isNumeric(name) {
		var err error
		if named, err = sctx.LookupTypeNamed(name, typ); err != nil {
			return nil, err
		}
		typ = named
	}
	a[name] = typ
	return named, nil
}

func isNumeric(s string) bool {
	for _, r := range s {
		if !isDigit(r) {
			return false
		}
	}
	return true
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func (a Analyzer) convertAny(sctx *super.Context, val ast.Any, cast super.Type) (Value, error) {
	// If we're casting something to a union, then the thing inside needs to
	// describe itself and we can convert the inner value to a union value when
	// we know its type (so we can code the tag).
	if union, ok := super.TypeUnder(cast).(*super.TypeUnion); ok {
		v, err := a.convertAny(sctx, val, nil)
		if err != nil {
			return nil, err
		}
		return a.convertUnion(sctx, v, union, cast)
	}
	switch val := val.(type) {
	case *ast.Primitive:
		return a.convertPrimitive(sctx, val, cast)
	case *ast.Record:
		return a.convertRecord(sctx, val, cast)
	case *ast.Array:
		return a.convertArray(sctx, val, cast)
	case *ast.Set:
		return a.convertSet(sctx, val, cast)
	case *ast.Enum:
		return a.convertEnum(sctx, val, cast)
	case *ast.Map:
		return a.convertMap(sctx, val, cast)
	case *ast.TypeValue:
		return a.convertTypeValue(sctx, val, cast)
	case *ast.Error:
		return a.convertError(sctx, val, cast)
	}
	return nil, fmt.Errorf("internal error: unknown ast type in Analyzer.convertAny: %T", val)
}

func (a Analyzer) convertPrimitive(sctx *super.Context, val *ast.Primitive, cast super.Type) (Value, error) {
	typ := super.LookupPrimitive(val.Type)
	if typ == nil {
		return nil, fmt.Errorf("no such primitive type: %q", val.Type)
	}
	isNull := typ == super.TypeNull
	if cast != nil {
		// The parser emits Enum values for identifiers but not for
		// string enum names.  Check if the cast type is an enum,
		// and if so, convert the string to its enum counterpart.
		if v := stringToEnum(val, cast); v != nil {
			return v, nil
		}
		var err error
		typ, err = castType(typ, cast)
		if err != nil {
			return nil, err
		}
	}
	if isNull {
		return &Null{Type: typ}, nil
	}
	return &Primitive{Type: typ, Text: val.Text}, nil
}

func stringToEnum(val *ast.Primitive, cast super.Type) Value {
	if enum, ok := cast.(*super.TypeEnum); ok {
		if val.Type == "string" {
			return &Enum{
				Type: enum,
				Name: val.Text,
			}
		}
	}
	return nil
}

func castType(typ, cast super.Type) (super.Type, error) {
	typID, castID := typ.ID(), cast.ID()
	if typID == castID || typID == super.IDNull ||
		super.IsInteger(typID) && (super.IsInteger(castID) || super.IsFloat(castID)) ||
		super.IsFloat(typID) && super.IsFloat(castID) {
		return cast, nil
	}
	return nil, fmt.Errorf("type mismatch: %q cannot be used as %q", FormatType(typ), FormatType(cast))
}

func (a Analyzer) convertRecord(sctx *super.Context, val *ast.Record, cast super.Type) (Value, error) {
	var fields []Value
	var err error
	if cast != nil {
		recType, ok := super.TypeUnder(cast).(*super.TypeRecord)
		if !ok {
			return nil, fmt.Errorf("record decorator not of type record: %T", cast)
		}
		if len(recType.Fields) != len(val.Fields) {
			return nil, fmt.Errorf("record decorator fields (%d) mismatched with value fields (%d)", len(recType.Fields), len(val.Fields))
		}
		fields, err = a.convertFields(sctx, val.Fields, recType.Fields)
	} else {
		fields, err = a.convertFields(sctx, val.Fields, nil)
		if err != nil {
			return nil, err
		}
		cast, err = lookupRecordType(sctx, val.Fields, fields)
	}
	if err != nil {
		return nil, err
	}
	return &Record{
		Type:   cast,
		Fields: fields,
	}, nil
}

func (a Analyzer) convertFields(sctx *super.Context, in []ast.Field, fields []super.Field) ([]Value, error) {
	vals := make([]Value, 0, len(in))
	for k, f := range in {
		var cast super.Type
		if fields != nil {
			cast = fields[k].Type
		}
		v, err := a.convertValue(sctx, f.Value, cast)
		if err != nil {
			return nil, err
		}
		vals = append(vals, v)
	}
	return vals, nil
}

func lookupRecordType(sctx *super.Context, in []ast.Field, vals []Value) (*super.TypeRecord, error) {
	fields := make([]super.Field, 0, len(in))
	for k, f := range in {
		fields = append(fields, super.Field{Name: f.Name, Type: vals[k].TypeOf()})
	}
	return sctx.LookupTypeRecord(fields)
}

// Figure out what the cast should be for the elements and for the union conversion if any.
func arrayElemCast(cast super.Type) (super.Type, error) {
	if cast == nil {
		return nil, nil
	}
	if arrayType, ok := super.TypeUnder(cast).(*super.TypeArray); ok {
		return arrayType.Type, nil
	}
	return nil, errors.New("array decorator not of type array")
}

func (a Analyzer) convertArray(sctx *super.Context, array *ast.Array, cast super.Type) (Value, error) {
	vals := make([]Value, 0, len(array.Elements))
	typ, err := arrayElemCast(cast)
	if err != nil {
		return nil, err
	}
	for _, elem := range array.Elements {
		v, err := a.convertValue(sctx, elem, typ)
		if err != nil {
			return nil, err
		}
		vals = append(vals, v)
	}
	if cast != nil || len(vals) == 0 {
		// We had a cast so we know any type mistmatches we have been
		// caught below...
		if cast == nil {
			cast = sctx.LookupTypeArray(super.TypeNull)
		}
		return &Array{
			Type:     cast,
			Elements: vals,
		}, nil
	}
	elems, inner, err := a.normalizeElems(sctx, vals)
	if err != nil {
		return nil, err
	}
	return &Array{
		Type:     sctx.LookupTypeArray(inner),
		Elements: elems,
	}, nil
}

func (a Analyzer) normalizeElems(sctx *super.Context, vals []Value) ([]Value, super.Type, error) {
	types := make([]super.Type, len(vals))
	for i, val := range vals {
		types[i] = val.TypeOf()
	}
	unique := types[:0]
	for _, typ := range super.UniqueTypes(types) {
		if typ != super.TypeNull {
			unique = append(unique, typ)
		}
	}
	if len(unique) == 1 {
		return vals, unique[0], nil
	}
	if len(unique) == 0 {
		return vals, super.TypeNull, nil
	}
	union := sctx.LookupTypeUnion(unique)
	var unions []Value
	for _, v := range vals {
		union, err := a.convertUnion(sctx, v, union, union)
		if err != nil {
			return nil, nil, err
		}
		unions = append(unions, union)
	}
	return unions, union, nil
}

func (a Analyzer) convertSet(sctx *super.Context, set *ast.Set, cast super.Type) (Value, error) {
	var elemType super.Type
	if cast != nil {
		setType, ok := super.TypeUnder(cast).(*super.TypeSet)
		if !ok {
			return nil, fmt.Errorf("set decorator not of type set: %T", cast)
		}
		elemType = setType.Type
	}
	vals := make([]Value, 0, len(set.Elements))
	for _, elem := range set.Elements {
		v, err := a.convertValue(sctx, elem, elemType)
		if err != nil {
			return nil, err
		}
		vals = append(vals, v)
	}
	if cast != nil || len(vals) == 0 {
		if cast == nil {
			cast = sctx.LookupTypeSet(super.TypeNull)
		}
		return &Array{
			Type:     cast,
			Elements: vals,
		}, nil
	}
	elems, inner, err := a.normalizeElems(sctx, vals)
	if err != nil {
		return nil, err
	}
	return &Set{
		Type:     sctx.LookupTypeSet(inner),
		Elements: elems,
	}, nil
}

func (a Analyzer) convertUnion(sctx *super.Context, v Value, union *super.TypeUnion, cast super.Type) (Value, error) {
	valType := v.TypeOf()
	if valType == super.TypeNull {
		// Set tag to -1 to signal to the builder to encode a null.
		return &Union{
			Type:  cast,
			Tag:   -1,
			Value: v,
		}, nil
	}
	for k, typ := range union.Types {
		if valType == typ {
			return &Union{
				Type:  cast,
				Tag:   k,
				Value: v,
			}, nil
		}
	}
	return nil, fmt.Errorf("type %q is not in union type %q", FormatType(valType), FormatType(union))
}

func (a Analyzer) convertEnum(sctx *super.Context, val *ast.Enum, cast super.Type) (Value, error) {
	if cast == nil {
		return nil, fmt.Errorf("identifier %q must be enum and requires decorator", val.Name)
	}
	enum, ok := super.TypeUnder(cast).(*super.TypeEnum)
	if !ok {
		return nil, fmt.Errorf("identifier %q is enum and incompatible with type %q", val.Name, FormatType(cast))
	}
	if slices.Contains(enum.Symbols, val.Name) {
		return &Enum{
			Name: val.Name,
			Type: cast,
		}, nil
	}
	return nil, fmt.Errorf("symbol %q not a member of type %q", val.Name, FormatType(enum))
}

func (a Analyzer) convertMap(sctx *super.Context, m *ast.Map, cast super.Type) (Value, error) {
	var keyType, valType super.Type
	if cast != nil {
		typ, ok := super.TypeUnder(cast).(*super.TypeMap)
		if !ok {
			return nil, errors.New("map decorator not of type map")
		}
		keyType = typ.KeyType
		valType = typ.ValType
	}
	keys := make([]Value, 0, len(m.Entries))
	vals := make([]Value, 0, len(m.Entries))
	for _, e := range m.Entries {
		key, err := a.convertValue(sctx, e.Key, keyType)
		if err != nil {
			return nil, err
		}
		val, err := a.convertValue(sctx, e.Value, valType)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
		vals = append(vals, val)
	}
	if cast == nil {
		// If there was no decorator, pull the types out of the first
		// entry we just analyed.
		if len(keys) == 0 {
			// empty set with no decorator
			keyType = super.TypeNull
			valType = super.TypeNull
		} else {
			var err error
			keys, keyType, err = a.normalizeElems(sctx, keys)
			if err != nil {
				return nil, err
			}
			vals, valType, err = a.normalizeElems(sctx, vals)
			if err != nil {
				return nil, err
			}
		}
		cast = sctx.LookupTypeMap(keyType, valType)
	}
	entries := make([]Entry, 0, len(keys))
	for i := range keys {
		entries = append(entries, Entry{keys[i], vals[i]})
	}
	return &Map{
		Type:    cast,
		Entries: entries,
	}, nil
}

func (a Analyzer) convertTypeValue(sctx *super.Context, tv *ast.TypeValue, cast super.Type) (Value, error) {
	if cast != nil {
		if _, ok := super.TypeUnder(cast).(*super.TypeOfType); !ok {
			return nil, fmt.Errorf("cannot apply decorator (%q) to a type value", FormatType(cast))
		}
	}
	typ, err := a.convertType(sctx, tv.Value)
	if err != nil {
		return nil, err
	}
	if cast == nil {
		cast = super.TypeType
	}
	return &TypeValue{
		Type:  cast,
		Value: typ,
	}, nil
}

func (a Analyzer) convertError(sctx *super.Context, val *ast.Error, cast super.Type) (Value, error) {
	var inner super.Type
	if cast != nil {
		typ, ok := super.TypeUnder(cast).(*super.TypeError)
		if !ok {
			return nil, errors.New("error decorator not of type error")
		}
		inner = typ.Type
	}
	under, err := a.convertValue(sctx, val.Value, inner)
	if err != nil {
		return nil, err
	}
	if cast == nil {
		cast = sctx.LookupTypeError(under.TypeOf())
	}
	return &Error{
		Value: under,
		Type:  cast,
	}, nil
}

func (a Analyzer) convertType(sctx *super.Context, typ ast.Type) (super.Type, error) {
	switch t := typ.(type) {
	case *ast.TypePrimitive:
		name := t.Name
		typ := super.LookupPrimitive(name)
		if typ == nil {
			return nil, fmt.Errorf("no such primitive type: %q", name)
		}
		return typ, nil
	case *ast.TypeDef:
		typ, err := a.convertType(sctx, t.Type)
		if err != nil {
			return nil, err
		}
		named, err := a.enterTypeDef(sctx, t.Name, typ)
		if err != nil {
			return nil, err
		}
		if named != nil {
			typ = named
		}
		return typ, nil
	case *ast.TypeRecord:
		return a.convertTypeRecord(sctx, t)
	case *ast.TypeArray:
		typ, err := a.convertType(sctx, t.Type)
		if err != nil {
			return nil, err
		}
		return sctx.LookupTypeArray(typ), nil
	case *ast.TypeSet:
		typ, err := a.convertType(sctx, t.Type)
		if err != nil {
			return nil, err
		}
		return sctx.LookupTypeSet(typ), nil
	case *ast.TypeMap:
		return a.convertTypeMap(sctx, t)
	case *ast.TypeUnion:
		return a.convertTypeUnion(sctx, t)
	case *ast.TypeEnum:
		return a.convertTypeEnum(sctx, t)
	case *ast.TypeError:
		typ, err := a.convertType(sctx, t.Type)
		if err != nil {
			return nil, err
		}
		return sctx.LookupTypeError(typ), nil
	case *ast.TypeName:
		typ, ok := a[t.Name]
		if !ok {
			// We avoid the nil-interface bug here by assigning to named
			// and then typ because assigning directly to typ will create
			// a nin-nil interface pointer for a nil result.
			named := sctx.LookupTypeDef(t.Name)
			if named == nil {
				return nil, fmt.Errorf("no such type name: %q", t.Name)
			}
			typ = named
		}
		return typ, nil
	}
	return nil, fmt.Errorf("unknown type in Analyzer.convertType: %T", typ)
}

func (a Analyzer) convertTypeRecord(sctx *super.Context, typ *ast.TypeRecord) (*super.TypeRecord, error) {
	fields := make([]super.Field, 0, len(typ.Fields))
	for _, f := range typ.Fields {
		typ, err := a.convertType(sctx, f.Type)
		if err != nil {
			return nil, err
		}
		fields = append(fields, super.Field{Name: f.Name, Type: typ})
	}
	return sctx.LookupTypeRecord(fields)
}

func (a Analyzer) convertTypeMap(sctx *super.Context, tmap *ast.TypeMap) (*super.TypeMap, error) {
	keyType, err := a.convertType(sctx, tmap.KeyType)
	if err != nil {
		return nil, err
	}
	valType, err := a.convertType(sctx, tmap.ValType)
	if err != nil {
		return nil, err
	}
	return sctx.LookupTypeMap(keyType, valType), nil
}

func (a Analyzer) convertTypeUnion(sctx *super.Context, union *ast.TypeUnion) (*super.TypeUnion, error) {
	var types []super.Type
	for _, typ := range union.Types {
		typ, err := a.convertType(sctx, typ)
		if err != nil {
			return nil, err
		}
		types = append(types, typ)
	}
	return sctx.LookupTypeUnion(types), nil
}

func (a Analyzer) convertTypeEnum(sctx *super.Context, enum *ast.TypeEnum) (*super.TypeEnum, error) {
	if len(enum.Symbols) == 0 {
		return nil, errors.New("enum body is empty")
	}
	return sctx.LookupTypeEnum(enum.Symbols), nil
}
