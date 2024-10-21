package super

import (
	"slices"
	"sync"
)

type Mapper struct {
	outputCtx *Context
	mu        sync.RWMutex
	types     []Type
}

func NewMapper(out *Context) *Mapper {
	return &Mapper{outputCtx: out}
}

// Lookup tranlates Zed types by type ID from one context to another.
// The first context is implied by the argument to Lookup() and the output
// type context is explicitly determined by the argument to NewMapper().
// If a binding has not yet been entered, nil is returned and Enter()
// should be called to create the binding.  There is a race here when two
// threads attempt to update the same ID, but it is safe because the
// outputContext will return the same the pointer so the second update
// does not change anything.
func (m *Mapper) Lookup(id int) Type {
	if id < IDTypeComplex {
		typ, _ := LookupPrimitiveByID(id)
		return typ
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if id < len(m.types) {
		return m.types[id]
	}
	return nil
}

func (m *Mapper) Enter(ext Type) (Type, error) {
	id := TypeID(ext)
	if id < IDTypeComplex {
		return LookupPrimitiveByID(id)
	}
	typ, err := m.outputCtx.TranslateType(ext)
	if err != nil {
		return nil, err
	}
	if typ != nil {
		m.EnterType(id, typ)
		return typ, nil
	}
	return nil, nil
}

func (m *Mapper) EnterType(id int, typ Type) {
	if id < IDTypeComplex {
		return
	}
	m.mu.Lock()
	if id >= len(m.types) {
		m.types = slices.Grow(m.types[:0], id+1)[:id+1]
	}
	m.types[id] = typ
	m.mu.Unlock()
}

// MapperLookupCache wraps a Mapper with an unsynchronized cache for its Lookup
// method.  Cache hits incur none of the synchronization overhead of
// Mapper.Lookup.
type MapperLookupCache struct {
	cache  []Type
	mapper *Mapper
}

func (m *MapperLookupCache) Reset(mapper *Mapper) {
	clear(m.cache)
	m.cache = m.cache[:0]
	m.mapper = mapper
}

func (m *MapperLookupCache) Lookup(id int) Type {
	if id < len(m.cache) {
		if typ := m.cache[id]; typ != nil {
			return typ
		}
	}
	typ := m.mapper.Lookup(id)
	if typ == nil {
		// To prevent OOM, don't grow cache if id is unknown.
		return nil
	}
	if id >= len(m.cache) {
		m.cache = slices.Grow(m.cache[:0], id+1)[:id+1]
	}
	m.cache[id] = typ
	return typ
}
