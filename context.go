package super

import (
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
	"unicode/utf8"

	"github.com/brimdata/super/scode"
)

const (
	MaxEnumSymbols  = 100_000
	MaxRecordFields = 100_000
	MaxUnionTypes   = 100_000
)

type TypeFetcher interface {
	LookupType(id int) (Type, error)
}

// A Context implements the "type context" in the super data model.  For a
// given set of related Values, each Value has a type from a shared Context.
// The Context manages the transitive closure of Types so that each unique
// type corresponds to exactly one Type pointer allowing type equivalence
// to be determined by pointer comparison.  (Type pointers from distinct
// Contexts obviously do not have this property.)
type Context struct {
	mu        sync.RWMutex
	byID      map[uint32]Type
	typedefs  *TypeDefs
	named     map[string]*TypeNamed
	stringErr atomic.Pointer[TypeError]
	toValue   map[Type]scode.Bytes
	toType    map[string]Type
}

var _ TypeFetcher = (*Context)(nil)

func NewContext() *Context {
	return &Context{
		byID:     make(map[uint32]Type),
		typedefs: NewTypeDefs(),
	}
}

func (c *Context) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.byID = make(map[uint32]Type)
	c.typedefs = NewTypeDefs()
	c.toValue = nil
	c.toType = nil
	c.named = nil
}

func (c *Context) TypeDefs() *TypeDefs {
	return c.typedefs
}

func (c *Context) LookupType(id int) (Type, error) {
	if id < 0 {
		return nil, fmt.Errorf("type id (%d) cannot be negative", id)
	}
	if id < IDTypeComplex {
		return LookupPrimitiveByID(id)
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	typ, ok := c.byID[uint32(id)]
	if !ok {
		return nil, fmt.Errorf("no type found for type id %d", id)
	}
	return typ, nil
}

type DuplicateFieldError struct {
	Name string
}

func (d *DuplicateFieldError) Error() string {
	return fmt.Sprintf("duplicate field: %q", d.Name)
}

// LookupTypeRecord returns a TypeRecord within this context that binds with the
// indicated fields.  Subsequent calls with the same fields will return the
// same record pointer.  If the type doesn't exist, it's created, stored,
// and returned.  The closure of types within the fields must all be from
// this type context.  If you want to use fields from a different type context,
// use TranslateTypeRecord.
func (c *Context) LookupTypeRecord(fields []Field) (*TypeRecord, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	id := c.typedefs.LookupTypeRecord(fields)
	if typ, ok := c.byID[id]; ok {
		return typ.(*TypeRecord), nil
	}
	if name, ok := duplicateField(fields); ok {
		return nil, &DuplicateFieldError{name}
	}
	typ := NewTypeRecord(int(id), slices.Clone(fields))
	c.byID[id] = typ
	return typ, nil
}

var namesPool = sync.Pool{
	New: func() any {
		// Return a pointer to avoid allocation on conversion to
		// interface.
		names := make([]string, 8)
		return &names
	},
}

func duplicateField(fields []Field) (string, bool) {
	if len(fields) < 2 {
		return "", false
	}
	names := namesPool.Get().(*[]string)
	defer namesPool.Put(names)
	*names = (*names)[:0]
	for _, f := range fields {
		*names = append(*names, f.Name)
	}
	sort.Strings(*names)
	prev := (*names)[0]
	for _, n := range (*names)[1:] {
		if n == prev {
			return n, true
		}
		prev = n
	}
	return "", false
}

func (c *Context) MustLookupTypeRecord(fields []Field) *TypeRecord {
	r, err := c.LookupTypeRecord(fields)
	if err != nil {
		panic(err)
	}
	return r
}

func (c *Context) LookupTypeArray(inner Type) *TypeArray {
	c.mu.Lock()
	defer c.mu.Unlock()
	id := c.typedefs.LookupTypeWrapped(TypeDefArray, inner)
	if typ, ok := c.byID[id]; ok {
		return typ.(*TypeArray)
	}
	typ := NewTypeArray(int(id), inner)
	c.byID[id] = typ
	return typ
}

func (c *Context) LookupTypeSet(inner Type) *TypeSet {
	c.mu.Lock()
	defer c.mu.Unlock()
	id := c.typedefs.LookupTypeWrapped(TypeDefSet, inner)
	if typ, ok := c.byID[id]; ok {
		return typ.(*TypeSet)
	}
	typ := NewTypeSet(int(id), inner)
	c.byID[id] = typ
	return typ
}

func (c *Context) LookupTypeMap(keyType, valType Type) *TypeMap {
	c.mu.Lock()
	defer c.mu.Unlock()
	id := c.typedefs.LookupTypeMap(keyType, valType)
	if typ, ok := c.byID[id]; ok {
		return typ.(*TypeMap)
	}
	typ := NewTypeMap(int(id), keyType, valType)
	c.byID[id] = typ
	return typ
}

func (c *Context) LookupTypeUnion(types []Type) (*TypeUnion, bool) {
	if badUnion(types) {
		return nil, false
	}
	sort.SliceStable(types, func(i, j int) bool {
		return CompareTypes(types[i], types[j]) < 0
	})
	c.mu.Lock()
	defer c.mu.Unlock()
	id := c.typedefs.LookupTypeUnion(types)
	if typ, ok := c.byID[id]; ok {
		return typ.(*TypeUnion), true
	}
	typ := NewTypeUnion(int(id), slices.Clone(types))
	c.byID[id] = typ
	return typ, true
}

func badUnion(types []Type) bool {
	for _, t := range types {
		if _, ok := t.(*TypeUnion); ok {
			return true
		}
	}
	return false
}

func (c *Context) LookupTypeEnum(symbols []string) *TypeEnum {
	c.mu.Lock()
	defer c.mu.Unlock()
	id := c.typedefs.LookupTypeEnum(symbols)
	if typ, ok := c.byID[id]; ok {
		return typ.(*TypeEnum)
	}
	typ := NewTypeEnum(int(id), symbols)
	c.byID[id] = typ
	return typ
}

// LookupByName returns the named type last bound to name by LookupTypeNamed.
// It returns nil if name is unbound.
func (c *Context) LookupByName(name string) *TypeNamed {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.named == nil {
		return nil
	}
	return c.named[name]
}

// LookupTypeNamed returns the named type for name and inner.  It also binds
// name to that named type.  LookupTypeNamed returns an error if name is not a
// valid UTF-8 string or is a primitive type name.
func (c *Context) LookupTypeNamed(name string, inner Type) (*TypeNamed, error) {
	if !utf8.ValidString(name) {
		return nil, fmt.Errorf("bad type name %q: invalid UTF-8", name)
	}
	if LookupPrimitive(name) != nil {
		return nil, fmt.Errorf("bad type name %q: primitive type name", name)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.named == nil {
		c.named = make(map[string]*TypeNamed)
	}
	id := c.typedefs.LookupTypeNamed(name, inner)
	if typ, ok := c.byID[id]; ok {
		named := typ.(*TypeNamed)
		c.named[name] = named
		return named, nil
	}
	typ := NewTypeNamed(int(id), name, inner)
	c.byID[id] = typ
	c.named[name] = typ
	return typ, nil
}

func (c *Context) LookupTypeError(inner Type) *TypeError {
	c.mu.Lock()
	defer c.mu.Unlock()
	id := c.typedefs.LookupTypeWrapped(TypeDefError, inner)
	if typ, ok := c.byID[id]; ok {
		return typ.(*TypeError)
	}
	typ := NewTypeError(int(id), inner)
	c.byID[id] = typ
	if inner == TypeString {
		c.stringErr.Store(typ)
	}
	return typ
}

func (c *Context) LookupTypeFusion(inner Type) *TypeFusion {
	c.mu.Lock()
	defer c.mu.Unlock()
	id := c.typedefs.LookupTypeWrapped(TypeDefFusion, inner)
	if typ, ok := c.byID[id]; ok {
		return typ.(*TypeFusion)
	}
	typ := NewTypeFusion(int(id), inner)
	c.byID[id] = typ
	return typ
}

// LookupByValue returns the Type indicated by a binary-serialized type value.
// This provides a means to translate a type-context-independent serialized
// encoding for an arbitrary type into the reciever Context.
func (c *Context) LookupByValue(tv scode.Bytes) (Type, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.toType == nil {
		c.toType = make(map[string]Type)
		c.toValue = make(map[Type]scode.Bytes)
	}
	typ, ok := c.toType[string(tv)]
	if ok {
		return typ, nil
	}
	c.mu.Unlock()
	typ, rest := c.DecodeTypeValue(tv)
	c.mu.Lock()
	if rest == nil {
		return nil, errors.New("bad type value encoding")
	}
	c.toValue[typ] = tv
	c.toType[string(tv)] = typ
	return typ, nil
}

// TranslateType takes a type from another context and creates and returns that
// type in this context.
func (c *Context) TranslateType(ext Type) (Type, error) {
	return c.LookupByValue(EncodeTypeValue(ext))
}

func (c *Context) LookupTypeValue(typ Type) Value {
	c.mu.Lock()
	if c.toValue != nil {
		if bytes, ok := c.toValue[typ]; ok {
			c.mu.Unlock()
			return NewValue(TypeType, bytes)
		}
	}
	c.mu.Unlock()
	tv := EncodeTypeValue(typ)
	typ, err := c.LookupByValue(tv)
	if err != nil {
		// This shouldn't happen.
		return c.Missing()
	}
	return c.LookupTypeValue(typ)
}

func (c *Context) DecodeTypeValue(tv scode.Bytes) (Type, scode.Bytes) {
	if len(tv) == 0 {
		return nil, nil
	}
	id := tv[0]
	tv = tv[1:]
	switch id {
	case TypeValueNameDef:
		name, tv := DecodeName(tv)
		if tv == nil {
			return nil, nil
		}
		var typ Type
		typ, tv = c.DecodeTypeValue(tv)
		if tv == nil {
			return nil, nil
		}
		named, err := c.LookupTypeNamed(name, typ)
		if err != nil {
			return nil, nil
		}
		return named, tv
	case TypeValueNameRef:
		name, tv := DecodeName(tv)
		if tv == nil {
			return nil, nil
		}
		typ := c.LookupByName(name)
		if typ == nil {
			return nil, nil
		}
		return typ, tv
	case TypeValueRecord:
		n, tv := DecodeLength(tv)
		if tv == nil || n > MaxRecordFields {
			return nil, nil
		}
		fields := make([]Field, 0, n)
		optlen := (n + 7) >> 3
		if optlen > len(tv) {
			return nil, nil
		}
		opts := tv[:optlen]
		tv = tv[optlen:]
		for k := range n {
			var name string
			name, tv = DecodeName(tv)
			if tv == nil {
				return nil, nil
			}
			var typ Type
			typ, tv = c.DecodeTypeValue(tv)
			if tv == nil {
				return nil, nil
			}
			fields = append(fields, Field{name, typ, scode.TestBit(opts, k)})
		}
		typ, err := c.LookupTypeRecord(fields)
		if err != nil {
			return nil, nil
		}
		return typ, tv
	case TypeValueArray:
		inner, tv := c.DecodeTypeValue(tv)
		if tv == nil {
			return nil, nil
		}
		typ := c.LookupTypeArray(inner)
		if typ == nil {
			return nil, nil
		}
		return typ, tv
	case TypeValueSet:
		inner, tv := c.DecodeTypeValue(tv)
		if tv == nil {
			return nil, nil
		}
		typ := c.LookupTypeSet(inner)
		if typ == nil {
			return nil, nil
		}
		return typ, tv
	case TypeValueMap:
		keyType, tv := c.DecodeTypeValue(tv)
		if tv == nil {
			return nil, nil
		}
		valType, tv := c.DecodeTypeValue(tv)
		if tv == nil {
			return nil, nil
		}
		typ := c.LookupTypeMap(keyType, valType)
		if typ == nil {
			return nil, nil
		}
		return typ, tv
	case TypeValueUnion:
		n, tv := DecodeLength(tv)
		if tv == nil || n > MaxUnionTypes {
			return nil, nil
		}
		types := make([]Type, 0, n)
		for range n {
			var typ Type
			typ, tv = c.DecodeTypeValue(tv)
			types = append(types, typ)
		}
		typ, ok := c.LookupTypeUnion(types)
		if typ == nil || !ok {
			return nil, nil
		}
		return typ, tv
	case TypeValueEnum:
		n, tv := DecodeLength(tv)
		if tv == nil || n > MaxEnumSymbols {
			return nil, nil
		}
		var symbols []string
		for range n {
			var symbol string
			symbol, tv = DecodeName(tv)
			if tv == nil {
				return nil, nil
			}
			symbols = append(symbols, symbol)
		}
		typ := c.LookupTypeEnum(symbols)
		if typ == nil {
			return nil, nil
		}
		return typ, tv
	case TypeValueError:
		inner, tv := c.DecodeTypeValue(tv)
		if tv == nil {
			return nil, nil
		}
		typ := c.LookupTypeError(inner)
		if typ == nil {
			return nil, nil
		}
		return typ, tv
	case TypeValueFusion:
		inner, tv := c.DecodeTypeValue(tv)
		if tv == nil {
			return nil, nil
		}
		typ := c.LookupTypeFusion(inner)
		if typ == nil {
			return nil, nil
		}
		return typ, tv
	default:
		typ, err := LookupPrimitiveByID(int(id))
		if err != nil {
			return nil, nil
		}
		return typ, tv
	}
}

func DecodeName(tv scode.Bytes) (string, scode.Bytes) {
	namelen, tv := DecodeLength(tv)
	if tv == nil || namelen > len(tv) {
		return "", nil
	}
	return string(tv[:namelen]), tv[namelen:]
}

func MustDecodeName(tv scode.Bytes) (string, scode.Bytes) {
	namelen, tv := MustDecodeLength(tv)
	return string(tv[:namelen]), tv[namelen:]
}

func DecodeLength(tv scode.Bytes) (int, scode.Bytes) {
	namelen, n := binary.Uvarint(tv)
	if n <= 0 {
		return 0, nil
	}
	return int(namelen), tv[n:]
}

func MustDecodeLength(tv scode.Bytes) (int, scode.Bytes) {
	namelen, n := binary.Uvarint(tv)
	if n <= 0 {
		panic(tv)
	}
	return int(namelen), tv[n:]
}

func DecodeID(b []byte) (uint32, []byte) {
	v, n := binary.Uvarint(b)
	if n <= 0 {
		return 0, nil
	}
	return uint32(v), b[n:]
}

func MustDecodeID(b []byte) (uint32, []byte) {
	v, n := binary.Uvarint(b)
	if n <= 0 {
		panic(b)
	}
	return uint32(v), b[n:]
}

func (c *Context) Missing() Value {
	return NewValue(c.StringTypeError(), Missing)
}

func (c *Context) Quiet() Value {
	return NewValue(c.StringTypeError(), Quiet)
}

// batch/allocator should handle these?

func (c *Context) NewErrorf(format string, args ...any) Value {
	return NewValue(c.StringTypeError(), fmt.Appendf(nil, format, args...))
}

func (c *Context) NewError(err error) Value {
	return NewValue(c.StringTypeError(), []byte(err.Error()))
}

func (c *Context) StringTypeError() *TypeError {
	if typ := c.stringErr.Load(); typ != nil {
		return typ
	}
	return c.LookupTypeError(TypeString)
}

func (c *Context) WrapError(msg string, val Value) Value {
	recType := c.MustLookupTypeRecord([]Field{
		{"message", TypeString, false},
		{"on", val.Type(), false},
	})
	errType := c.LookupTypeError(recType)
	var b scode.Builder
	b.Append(EncodeString(msg))
	b.Append(val.Bytes())
	return NewValue(errType, b.Bytes())
}

func (c *Context) Nullable(typ Type) *TypeUnion {
	var types []Type
	if union, ok := typ.(*TypeUnion); ok {
		for _, t := range union.Types {
			if t == TypeNull {
				return union
			}
		}
		types = slices.Clone(union.Types)
	} else {
		types = []Type{typ}
	}
	out, ok := c.LookupTypeUnion(append(types, TypeNull))
	if !ok {
		panic(typ)
	}
	return out
}

func NullableUnion(typ Type) (*TypeUnion, int) {
	if union, ok := typ.(*TypeUnion); ok {
		for tag, typ := range union.Types {
			if typ == TypeNull {
				return union, tag
			}
		}
	}
	return nil, 0
}

// TypeCache wraps a TypeFetcher with an unsynchronized cache for its LookupType
// method.  Cache hits incur none of the synchronization overhead of
// the underlying shared type context.
type TypeCache struct {
	cache   []Type
	fetcher TypeFetcher
}

var _ TypeFetcher = (*TypeCache)(nil)

func (t *TypeCache) Reset(fetcher TypeFetcher) {
	clear(t.cache)
	t.cache = t.cache[:0]
	t.fetcher = fetcher
}

func (t *TypeCache) LookupType(id int) (Type, error) {
	if id < len(t.cache) {
		if typ := t.cache[id]; typ != nil {
			return typ, nil
		}
	}
	typ, err := t.fetcher.LookupType(id)
	if err != nil {
		return nil, err
	}
	if id >= len(t.cache) {
		t.cache = slices.Grow(t.cache[:0], id+1)[:id+1]
	}
	t.cache[id] = typ
	return typ, nil
}

const (
	TypeDefRecord = 0
	TypeDefArray  = 1
	TypeDefSet    = 2
	TypeDefMap    = 3
	TypeDefUnion  = 4
	TypeDefEnum   = 5
	TypeDefError  = 6
	TypeDefNamed  = 7
	TypeDefFusion = 8
)

// TypeDefs encodes an interned set of types using type IDs that are
// local to this data structure.  This is used by Context to hold
// its type system and by fusion types and type values that implement
// vector.TypeLoader so that types may be materialized into the query
// Context on demand only when needed.  This data structure is designed
// to be serialized and deserialized as a whole into CSUP and BSUP formats.
type TypeDefs struct {
	offsets []uint32
	bytes   []byte
	lut     map[string]uint32
}

func NewTypeDefs() *TypeDefs {
	return &TypeDefs{
		offsets: make([]uint32, 1),
		lut:     make(map[string]uint32),
	}
}

func (t *TypeDefs) Reset() {
	t.bytes = t.bytes[:0]
	t.offsets = t.offsets[:1]
	t.lut = make(map[string]uint32)
}

func (t *TypeDefs) Bytes() []byte {
	return t.bytes
}

func (t *TypeDefs) Serialization(id uint32) []byte {
	slot := id - IDTypeComplex
	return t.bytes[t.offsets[slot]:t.offsets[slot+1]]
}

func (t *TypeDefs) NTypes() int {
	return len(t.offsets) - 1
}

func (t *TypeDefs) Append(bytes []byte) uint32 {
	slot := uint32(len(t.offsets) - 1)
	t.bytes = append(t.bytes, bytes...)
	t.offsets = append(t.offsets, uint32(len(t.bytes)))
	return slot + IDTypeComplex
}

func (t *TypeDefs) AppendInPlace() uint32 {
	// Do an append but use the bytes that are poking off
	// the end as the new value and compute the slot from that.
	slot := uint32(len(t.offsets) - 1)
	t.offsets = append(t.offsets, uint32(len(t.bytes)))
	return slot + IDTypeComplex
}

func (t *TypeDefs) Lookup(at int) uint32 {
	key := string(t.bytes[at:])
	id, ok := t.lut[key]
	if !ok {
		id = t.AppendInPlace()
		t.lut[key] = id
	} else {
		t.bytes = t.bytes[:at]
	}
	return id
}

func (t *TypeDefs) LookupType(ext Type) uint32 {
	if id := TypeID(ext); id < IDTypeComplex {
		return uint32(id)
	}
	switch ext := ext.(type) {
	case *TypeRecord:
		return t.LookupTypeRecord(ext.Fields)
	case *TypeArray:
		return t.LookupTypeWrapped(TypeDefArray, ext.Type)
	case *TypeSet:
		return t.LookupTypeWrapped(TypeDefSet, ext.Type)
	case *TypeMap:
		return t.LookupTypeMap(ext.KeyType, ext.ValType)
	case *TypeUnion:
		return t.LookupTypeUnion(ext.Types)
	case *TypeEnum:
		return t.LookupTypeEnum(ext.Symbols)
	case *TypeError:
		return t.LookupTypeWrapped(TypeDefError, ext.Type)
	case *TypeNamed:
		return t.LookupTypeNamed(ext.Name, ext.Type)
	case *TypeFusion:
		return t.LookupTypeWrapped(TypeDefFusion, ext.Type)
	default:
		panic(ext)
	}
}

func (t *TypeDefs) LookupTypeRecord(fields []Field) uint32 {
	// XXX change this to use pool for ids if profiling warrants
	var ids []uint32
	for _, f := range fields {
		ids = append(ids, t.LookupType(f.Type))
	}
	at := len(t.bytes)
	t.bytes = append(t.bytes, TypeDefRecord)
	t.bytes = binary.AppendUvarint(t.bytes, uint64(len(fields)))
	for k, f := range fields {
		t.bytes = binary.AppendUvarint(t.bytes, uint64(len(f.Name)))
		t.bytes = append(t.bytes, f.Name...)
		t.bytes = binary.AppendUvarint(t.bytes, uint64(ids[k]))
		var opt byte
		if f.Opt {
			opt = 1
		}
		t.bytes = append(t.bytes, opt)
	}
	return t.Lookup(at)
}

func (t *TypeDefs) BindTypeRecord(names []string, fields []uint32, opts []bool) uint32 {
	at := len(t.bytes)
	t.bytes = append(t.bytes, TypeDefRecord)
	t.bytes = binary.AppendUvarint(t.bytes, uint64(len(fields)))
	for k, id := range fields {
		t.bytes = binary.AppendUvarint(t.bytes, uint64(len(names[k])))
		t.bytes = append(t.bytes, names[k]...)
		t.bytes = binary.AppendUvarint(t.bytes, uint64(id))
		var opt byte
		if opts != nil && opts[k] {
			opt = 1
		}
		t.bytes = append(t.bytes, opt)
	}
	return t.Lookup(at)
}

func (t *TypeDefs) LookupTypeWrapped(typedef int, inner Type) uint32 {
	id := t.LookupType(inner)
	at := len(t.bytes)
	t.bytes = append(t.bytes, byte(typedef))
	t.bytes = binary.AppendUvarint(t.bytes, uint64(id))
	return t.Lookup(at)
}

func (t *TypeDefs) BindTypeWrapped(typedef int, inner uint32) uint32 {
	at := len(t.bytes)
	t.bytes = append(t.bytes, byte(typedef))
	t.bytes = binary.AppendUvarint(t.bytes, uint64(inner))
	return t.Lookup(at)
}

func (t *TypeDefs) LookupTypeMap(keyType, valType Type) uint32 {
	keyID := t.LookupType(keyType)
	valID := t.LookupType(valType)
	at := len(t.bytes)
	t.bytes = append(t.bytes, TypeDefMap)
	t.bytes = binary.AppendUvarint(t.bytes, uint64(keyID))
	t.bytes = binary.AppendUvarint(t.bytes, uint64(valID))
	return t.Lookup(at)
}

func (t *TypeDefs) LookupTypeUnion(types []Type) uint32 {
	sort.SliceStable(types, func(i, j int) bool {
		return CompareTypes(types[i], types[j]) < 0
	})
	// XXX change this to use pool for ids if profiling warrants
	var ids []uint32
	for _, typ := range types {
		id := t.LookupType(typ)
		ids = append(ids, id)
	}
	at := len(t.bytes)
	t.bytes = append(t.bytes, TypeDefUnion)
	t.bytes = binary.AppendUvarint(t.bytes, uint64(len(ids)))
	for _, id := range ids {
		t.bytes = binary.AppendUvarint(t.bytes, uint64(id))
	}
	return t.Lookup(at)
}

func (t *TypeDefs) LookupTypeEnum(symbols []string) uint32 {
	at := len(t.bytes)
	t.bytes = append(t.bytes, TypeDefEnum)
	t.bytes = binary.AppendUvarint(t.bytes, uint64(len(symbols)))
	for _, s := range symbols {
		t.bytes = binary.AppendUvarint(t.bytes, uint64(len(s)))
		t.bytes = append(t.bytes, s...)
	}
	return t.Lookup(at)
}

func (t *TypeDefs) LookupTypeNamed(name string, inner Type) uint32 {
	id := t.LookupType(inner)
	at := len(t.bytes)
	t.bytes = append(t.bytes, TypeDefNamed)
	t.bytes = binary.AppendUvarint(t.bytes, uint64(len(name)))
	t.bytes = append(t.bytes, name...)
	t.bytes = binary.AppendUvarint(t.bytes, uint64(id))
	return t.Lookup(at)
}

type TypeDefsMapper struct {
	*TypeDefs
	sctx  *Context
	cache map[uint32]Type
}

func NewTypeDefsMapper(sctx *Context, defs *TypeDefs) *TypeDefsMapper {
	return &TypeDefsMapper{
		TypeDefs: defs,
		sctx:     sctx,
		cache:    make(map[uint32]Type),
	}
}

func (t *TypeDefsMapper) LookupType(id uint32) Type {
	typ, ok := t.cache[id]
	if !ok {
		typ = t.lookupType(id)
		t.cache[id] = typ
	}
	return typ
}

func (t *TypeDefsMapper) lookupType(id uint32) Type {
	if id < IDTypeComplex {
		t, _ := LookupPrimitiveByID(int(id))
		return t
	}
	b := t.Serialization(id)
	typedef := b[0]
	b = b[1:]
	switch typedef {
	case TypeDefNamed:
		name, b := DecodeName(b)
		if b == nil {
			return nil
		}
		id, b := DecodeID(b)
		typ := t.LookupType(id)
		if typ == nil {
			return nil
		}
		named, err := t.sctx.LookupTypeNamed(name, typ)
		if err != nil {
			return nil
		}
		return named
	case TypeDefRecord:
		n, b := DecodeLength(b)
		if b == nil || n > MaxRecordFields {
			return nil
		}
		fields := make([]Field, 0, n)
		for range n {
			var name string
			name, b = DecodeName(b)
			if b == nil {
				return nil
			}
			var id uint32
			id, b = DecodeID(b)
			typ := t.LookupType(id)
			if typ == nil {
				return nil
			}
			optByte := b[0]
			b = b[1:]
			var opt bool
			if optByte != 0 {
				opt = true
			}
			fields = append(fields, Field{name, typ, opt})
		}
		typ, err := t.sctx.LookupTypeRecord(fields)
		if err != nil {
			return nil
		}
		return typ
	case TypeDefArray:
		var id uint32
		id, _ = DecodeID(b)
		inner := t.LookupType(id)
		if inner == nil {
			return nil
		}
		return t.sctx.LookupTypeArray(inner)
	case TypeDefSet:
		var id uint32
		id, _ = DecodeID(b)
		inner := t.LookupType(id)
		if inner == nil {
			return nil
		}
		return t.sctx.LookupTypeSet(inner)
	case TypeDefMap:
		var keyID, valID uint32
		keyID, b = DecodeID(b)
		if b == nil {
			return nil
		}
		valID, _ = DecodeID(b)
		keyType := t.LookupType(keyID)
		valType := t.LookupType(valID)
		if keyType == nil || valType == nil {
			return nil
		}
		return t.sctx.LookupTypeMap(keyType, valType)
	case TypeDefUnion:
		n, b := DecodeLength(b)
		if b == nil || n > MaxUnionTypes {
			return nil
		}
		types := make([]Type, 0, n)
		for range n {
			var id uint32
			id, b = DecodeID(b)
			typ := t.LookupType(id)
			if typ == nil {
				return nil
			}
			types = append(types, typ)
		}
		typ, ok := t.sctx.LookupTypeUnion(types)
		if !ok {
			return nil
		}
		return typ
	case TypeDefEnum:
		n, b := DecodeLength(b)
		if b == nil || n > MaxEnumSymbols {
			return nil
		}
		var symbols []string
		for range n {
			var symbol string
			symbol, b = DecodeName(b)
			if b == nil {
				return nil
			}
			symbols = append(symbols, symbol)
		}
		return t.sctx.LookupTypeEnum(symbols)
	case TypeDefError:
		id, b := DecodeID(b)
		if b == nil {
			return nil
		}
		inner := t.LookupType(id)
		if inner == nil {
			return nil
		}
		return t.sctx.LookupTypeError(inner)
	case TypeDefFusion:
		id, b := DecodeID(b)
		if b == nil {
			return nil
		}
		inner := t.LookupType(id)
		if inner == nil {
			return nil
		}
		return t.sctx.LookupTypeFusion(inner)
	default:
		panic(id)
	}
}

// A TypeDefsMerger recodes typedefs from an external table to a shared table
// on demand as external ID are looked up and converted to shared IDs.  This is
// used, for example, by the CSUP writer to collapse multiple typedefs tables
// into one table to be written to the CSUP metadata and copying only the typedefs
// that are used by subtypes in the serialized fusion vectors.  LookupID panics if any
// malformed data is encountered.
type TypeDefsMerger struct {
	*TypeDefs
	ext   *TypeDefs
	idmap map[uint32]uint32
}

func NewTypeDefsMerger(defs, ext *TypeDefs) *TypeDefsMerger {
	return &TypeDefsMerger{
		TypeDefs: defs,
		ext:      ext,
		idmap:    make(map[uint32]uint32),
	}
}

func (t *TypeDefsMerger) LookupID(extID uint32) uint32 {
	if extID < IDTypeComplex {
		return extID
	}
	if id, ok := t.idmap[extID]; ok {
		return id
	}
	bytes := t.ext.Serialization(extID)
	typedef := bytes[0]
	bytes = bytes[1:]
	var id uint32
	var n, at int
	var name string
	switch typedef {
	case TypeDefNamed:
		name, bytes = MustDecodeName(bytes)
		id, bytes = MustDecodeID(bytes)
		id = t.LookupID(id)
		at = len(t.bytes)
		t.bytes = append(t.bytes, TypeDefNamed)
		t.bytes = binary.AppendUvarint(t.bytes, uint64(len(name)))
		t.bytes = append(t.bytes, name...)
		t.bytes = binary.AppendUvarint(t.bytes, uint64(id))
	case TypeDefRecord:
		// extra alloc and copy due to recursion.  XXX put this in a pool.
		var out []byte
		n, bytes = MustDecodeLength(bytes)
		if n > MaxRecordFields {
			panic(n)
		}
		out = append(out, TypeDefRecord)
		out = binary.AppendUvarint(out, uint64(n))
		for range n {
			var name string
			name, bytes = MustDecodeName(bytes)
			id, bytes = MustDecodeID(bytes)
			opt := bytes[0]
			bytes = bytes[1:]
			out = binary.AppendUvarint(out, uint64(len(name)))
			out = append(out, name...)
			out = binary.AppendUvarint(out, uint64(t.LookupID(id)))
			out = append(out, opt)
		}
		at = len(t.bytes)
		t.bytes = append(t.bytes, out...)
	case TypeDefArray:
		id, bytes = MustDecodeID(bytes)
		id = t.LookupID(id)
		at = len(t.bytes)
		t.bytes = append(t.bytes, TypeDefArray)
		t.bytes = binary.AppendUvarint(t.bytes, uint64(id))
	case TypeDefSet:
		id, bytes = MustDecodeID(bytes)
		id = t.LookupID(id)
		at = len(t.bytes)
		t.bytes = append(t.bytes, TypeDefSet)
		t.bytes = binary.AppendUvarint(t.bytes, uint64(id))
	case TypeDefMap:
		id, bytes = MustDecodeID(bytes)
		keyID := t.LookupID(id)
		id, bytes = MustDecodeID(bytes)
		valID := t.LookupID(id)
		at = len(t.bytes)
		t.bytes = append(t.bytes, TypeDefMap)
		t.bytes = binary.AppendUvarint(t.bytes, uint64(keyID))
		t.bytes = binary.AppendUvarint(t.bytes, uint64(valID))
	case TypeDefUnion:
		// extra alloc and copy due to recursion.  XXX put this in a pool.
		var out []byte
		n, bytes = MustDecodeLength(bytes)
		if n > MaxUnionTypes {
			panic(n)
		}
		out = append(out, TypeDefUnion)
		out = binary.AppendUvarint(out, uint64(n))
		for range n {
			id, bytes = MustDecodeID(bytes)
			out = binary.AppendUvarint(out, uint64(t.LookupID(id)))
		}
		at = len(t.bytes)
		t.bytes = append(t.bytes, out...)
	case TypeDefEnum:
		n, bytes = MustDecodeLength(bytes)
		at = len(t.bytes)
		t.bytes = append(t.bytes, TypeDefEnum)
		t.bytes = binary.AppendUvarint(t.bytes, uint64(n))
		for range n {
			name, bytes = MustDecodeName(bytes)
			t.bytes = binary.AppendUvarint(t.bytes, uint64(len(name)))
			t.bytes = append(t.bytes, name...)
		}
	case TypeDefError:
		id, bytes = MustDecodeID(bytes)
		id = t.LookupID(id)
		at = len(t.bytes)
		t.bytes = append(t.bytes, TypeDefError)
		t.bytes = binary.AppendUvarint(t.bytes, uint64(id))
	case TypeDefFusion:
		id, bytes = MustDecodeID(bytes)
		id = t.LookupID(id)
		at = len(t.bytes)
		t.bytes = append(t.bytes, TypeDefFusion)
		t.bytes = binary.AppendUvarint(t.bytes, uint64(id))
	default:
		panic(id)
	}
	id = t.Lookup(at)
	t.idmap[extID] = id
	return id
}

// NewTypeDefsFromBytes takes a serialized representation of a typedefs table
// and computes the lookup table and offsets of each typedef in the table, returning
// a new TypeDefs table.  It checks that all referenced IDs in the table maintain
// the invariant that they are defined before use in the scan order of the table.
// It panics if malformed data is encountered.
func NewTypeDefsFromBytes(bytes []byte) *TypeDefs {
	defs := NewTypeDefs()
	defs.bytes = bytes
	localID := uint32(IDTypeComplex)
	var off uint32
	for len(bytes) > 0 {
		before := bytes
		typedef := bytes[0]
		bytes = bytes[1:]
		var id uint32
		var n int
		switch typedef {
		case TypeDefNamed:
			_, bytes = MustDecodeName(bytes)
			id, bytes = MustDecodeID(bytes)
			if id >= localID {
				panic(id)
			}
		case TypeDefRecord:
			n, bytes = MustDecodeLength(bytes)
			if n > MaxRecordFields {
				panic(n)
			}
			for range n {
				_, bytes = MustDecodeName(bytes)
				id, bytes = MustDecodeID(bytes)
				if id >= localID {
					panic(id)
				}
				// field opt
				bytes = bytes[1:]
			}
		case TypeDefArray, TypeDefSet, TypeDefError, TypeDefFusion:
			id, bytes = MustDecodeID(bytes)
			if id >= localID {
				panic(id)
			}
		case TypeDefMap:
			// key ID
			id, bytes = MustDecodeID(bytes)
			if id >= localID {
				panic(id)
			}
			// val ID
			id, bytes = MustDecodeID(bytes)
			if id >= localID {
				panic(id)
			}
		case TypeDefUnion:
			n, bytes = MustDecodeLength(bytes)
			if n > MaxUnionTypes {
				panic(n)
			}
			for range n {
				id, bytes = MustDecodeID(bytes)
				if id >= localID {
					panic(id)
				}
			}
		case TypeDefEnum:
			n, bytes = MustDecodeLength(bytes)
			if n > MaxEnumSymbols {
				panic(n)
			}
			for range n {
				_, bytes = MustDecodeName(bytes)
			}
		default:
			panic(typedef)
		}
		size := len(before) - len(bytes)
		off += uint32(size)
		defs.lut[string(before[:size])] = localID
		defs.offsets = append(defs.offsets, off)
		localID++
	}
	return defs
}
