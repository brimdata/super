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

	"github.com/brimdata/super/zcode"
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
	byID      []Type
	typedefs  map[string]*TypeNamed
	stringErr atomic.Pointer[TypeError]
	recs      map[string]*TypeRecord
	arrays    map[Type]*TypeArray
	sets      map[Type]*TypeSet
	maps      map[string]*TypeMap
	unions    map[string]*TypeUnion
	enums     map[string]*TypeEnum
	nameds    map[string]*TypeNamed
	errors    map[Type]*TypeError
	toValue   map[Type]zcode.Bytes
	toType    map[string]Type
}

var _ TypeFetcher = (*Context)(nil)

func NewContext() *Context {
	return &Context{
		byID: make([]Type, IDTypeComplex, 2*IDTypeComplex),
	}
}

func (c *Context) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.byID = c.byID[:IDTypeComplex]
	c.toValue = nil
	c.toType = nil
	c.recs = nil
	c.arrays = nil
	c.sets = nil
	c.maps = nil
	c.unions = nil
	c.enums = nil
	c.nameds = nil
	c.errors = nil
	c.typedefs = nil
}

func (c *Context) nextIDWithLock() int {
	return len(c.byID)
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
	if id >= len(c.byID) {
		return nil, fmt.Errorf("type id (%d) not in type context (size %d)", id, len(c.byID))
	}
	if typ := c.byID[id]; typ != nil {
		return typ, nil
	}
	return nil, fmt.Errorf("no type found for type id %d", id)
}

var keyPool = sync.Pool{
	New: func() interface{} {
		// Return a pointer to avoid allocation on conversion to
		// interface.
		buf := make([]byte, 64)
		return &buf
	},
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
	key := keyPool.Get().(*[]byte)
	bytes := (*key)[:0]
	for _, field := range fields {
		bytes = binary.LittleEndian.AppendUint32(bytes, uint32(len(field.Name)))
		bytes = append(bytes, field.Name...)
		bytes = binary.LittleEndian.AppendUint32(bytes, uint32(TypeID(field.Type)))
	}
	*key = bytes
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.recs == nil {
		c.recs = make(map[string]*TypeRecord)
	}
	if typ, ok := c.recs[string(bytes)]; ok {
		keyPool.Put(key)
		return typ, nil
	}
	if name, ok := duplicateField(fields); ok {
		return nil, &DuplicateFieldError{name}
	}
	typ := NewTypeRecord(c.nextIDWithLock(), slices.Clone(fields))
	c.recs[string(bytes)] = typ
	c.enterWithLock(typ)
	return typ, nil
}

var namesPool = sync.Pool{
	New: func() interface{} {
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
	if c.arrays == nil {
		c.arrays = make(map[Type]*TypeArray)
	}
	if typ, ok := c.arrays[inner]; ok {
		return typ
	}
	typ := NewTypeArray(c.nextIDWithLock(), inner)
	c.enterWithLock(typ)
	c.arrays[inner] = typ
	return typ
}

func (c *Context) LookupTypeSet(inner Type) *TypeSet {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sets == nil {
		c.sets = make(map[Type]*TypeSet)
	}
	if typ, ok := c.sets[inner]; ok {
		return typ
	}
	typ := NewTypeSet(c.nextIDWithLock(), inner)
	c.enterWithLock(typ)
	c.sets[inner] = typ
	return typ
}

func (c *Context) LookupTypeMap(keyType, valType Type) *TypeMap {
	key := keyPool.Get().(*[]byte)
	bytes := (*key)[:0]
	bytes = binary.LittleEndian.AppendUint32(bytes, uint32(TypeID(keyType)))
	bytes = binary.LittleEndian.AppendUint32(bytes, uint32(TypeID(valType)))
	*key = bytes
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.maps == nil {
		c.maps = make(map[string]*TypeMap)
	}
	if typ, ok := c.maps[string(bytes)]; ok {
		keyPool.Put(key)
		return typ
	}
	typ := NewTypeMap(c.nextIDWithLock(), keyType, valType)
	c.enterWithLock(typ)
	c.maps[string(bytes)] = typ
	return typ
}

func (c *Context) LookupTypeUnion(types []Type) *TypeUnion {
	sort.SliceStable(types, func(i, j int) bool {
		return CompareTypes(types[i], types[j]) < 0
	})
	key := keyPool.Get().(*[]byte)
	bytes := (*key)[:0]
	for _, typ := range types {
		bytes = binary.LittleEndian.AppendUint32(bytes, uint32(TypeID(typ)))
	}
	*key = bytes
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.unions == nil {
		c.unions = make(map[string]*TypeUnion)
	}
	if typ, ok := c.unions[string(bytes)]; ok {
		keyPool.Put(key)
		return typ
	}
	typ := NewTypeUnion(c.nextIDWithLock(), slices.Clone(types))
	c.enterWithLock(typ)
	c.unions[string(bytes)] = typ
	return typ
}

func (c *Context) LookupTypeEnum(symbols []string) *TypeEnum {
	key := keyPool.Get().(*[]byte)
	bytes := (*key)[:0]
	for _, symbol := range symbols {
		bytes = binary.LittleEndian.AppendUint32(bytes, uint32(len(symbol)))
		bytes = append(bytes, symbol...)
	}
	*key = bytes
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.enums == nil {
		c.enums = make(map[string]*TypeEnum)
	}
	if typ, ok := c.enums[string(bytes)]; ok {
		keyPool.Put(key)
		return typ
	}
	typ := NewTypeEnum(c.nextIDWithLock(), slices.Clone(symbols))
	c.enterWithLock(typ)
	c.enums[string(bytes)] = typ
	return typ
}

// LookupTypeDef returns the named type last bound to name by LookupTypeNamed.
// It returns nil if name is unbound.
func (c *Context) LookupTypeDef(name string) *TypeNamed {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.typedefs == nil {
		return nil
	}
	return c.typedefs[name]
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
	key := keyPool.Get().(*[]byte)
	bytes := (*key)[:0]
	bytes = binary.LittleEndian.AppendUint32(bytes, uint32(len(name)))
	bytes = append(bytes, name...)
	bytes = binary.LittleEndian.AppendUint32(bytes, uint32(TypeID(inner)))
	*key = bytes
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.nameds == nil {
		c.nameds = make(map[string]*TypeNamed)
		c.typedefs = make(map[string]*TypeNamed)
	}
	if typ, ok := c.nameds[string(bytes)]; ok {
		keyPool.Put(key)
		c.typedefs[name] = typ
		return typ, nil
	}
	typ := NewTypeNamed(c.nextIDWithLock(), name, inner)
	c.typedefs[name] = typ
	c.enterWithLock(typ)
	c.nameds[string(bytes)] = typ
	return typ, nil
}

func (c *Context) LookupTypeError(inner Type) *TypeError {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.errors == nil {
		c.errors = make(map[Type]*TypeError)
	}
	if typ, ok := c.errors[inner]; ok {
		return typ
	}
	typ := NewTypeError(c.nextIDWithLock(), inner)
	c.enterWithLock(typ)
	c.errors[inner] = typ
	if inner == TypeString {
		c.stringErr.Store(typ)
	}
	return typ
}

// LookupByValue returns the Type indicated by a binary-serialized type value.
// This provides a means to translate a type-context-independent serialized
// encoding for an arbitrary type into the reciever Context.
func (c *Context) LookupByValue(tv zcode.Bytes) (Type, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.toType == nil {
		c.toType = make(map[string]Type)
		c.toValue = make(map[Type]zcode.Bytes)
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

func (c *Context) enterWithLock(typ Type) {
	c.byID = append(c.byID, typ)
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

func (c *Context) DecodeTypeValue(tv zcode.Bytes) (Type, zcode.Bytes) {
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
		typ := c.LookupTypeDef(name)
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
		for k := 0; k < n; k++ {
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
			fields = append(fields, Field{name, typ})
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
		for k := 0; k < n; k++ {
			var typ Type
			typ, tv = c.DecodeTypeValue(tv)
			types = append(types, typ)
		}
		typ := c.LookupTypeUnion(types)
		if typ == nil {
			return nil, nil
		}
		return typ, tv
	case TypeValueEnum:
		n, tv := DecodeLength(tv)
		if tv == nil || n > MaxEnumSymbols {
			return nil, nil
		}
		var symbols []string
		for k := 0; k < n; k++ {
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
	default:
		typ, err := LookupPrimitiveByID(int(id))
		if err != nil {
			return nil, nil
		}
		return typ, tv
	}
}

func DecodeName(tv zcode.Bytes) (string, zcode.Bytes) {
	namelen, tv := DecodeLength(tv)
	if tv == nil || namelen > len(tv) {
		return "", nil
	}
	return string(tv[:namelen]), tv[namelen:]
}

func DecodeLength(tv zcode.Bytes) (int, zcode.Bytes) {
	namelen, n := binary.Uvarint(tv)
	if n <= 0 {
		return 0, nil
	}
	return int(namelen), tv[n:]
}

func (c *Context) Missing() Value {
	return NewValue(c.StringTypeError(), Missing)
}

func (c *Context) Quiet() Value {
	return NewValue(c.StringTypeError(), Quiet)
}

// batch/allocator should handle these?

func (c *Context) NewErrorf(format string, args ...interface{}) Value {
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
		{"message", TypeString},
		{"on", val.Type()},
	})
	errType := c.LookupTypeError(recType)
	var b zcode.Builder
	b.Append(EncodeString(msg))
	b.Append(val.Bytes())
	return NewValue(errType, b.Bytes())
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
