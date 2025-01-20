package agg

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
)

type Union struct {
	types map[super.Type]map[string]struct{}
	size  int
}

var _ Function = (*Union)(nil)

func NewUnion() *Union {
	return &Union{
		types: make(map[super.Type]map[string]struct{}),
	}
}

func (u *Union) Consume(val super.Value) {
	if val.IsNull() {
		return
	}
	u.Update(val.Type(), val.Bytes())
}

func (u *Union) Update(typ super.Type, b zcode.Bytes) {
	m, ok := u.types[typ]
	if !ok {
		m = make(map[string]struct{})
		u.types[typ] = m
	}
	if _, ok := m[string(b)]; !ok {
		m[string(b)] = struct{}{}
		u.size += len(b)
		for u.size > MaxValueSize {
			u.deleteOne()
			// XXX See issue #1813.  For now, we silently discard
			// entries to maintain the size limit.
			//return ErrRowTooBig
		}
	}
}

func (u *Union) deleteOne() {
	for typ, m := range u.types {
		for key := range m {
			u.size -= len(key)
			delete(m, key)
			if len(m) == 0 {
				delete(u.types, typ)
			}
			return
		}
	}
}

func (u *Union) Result(zctx *super.Context) super.Value {
	if len(u.types) == 0 {
		return super.Null
	}
	types := make([]super.Type, 0, len(u.types))
	for typ := range u.types {
		types = append(types, typ)
	}
	var inner super.Type
	var b zcode.Builder
	if len(types) > 1 {
		union := zctx.LookupTypeUnion(types)
		inner = union
		for typ, m := range u.types {
			for v := range m {
				super.BuildUnion(&b, union.TagOf(typ), []byte(v))
			}
		}
	} else {
		inner = types[0]
		for v := range u.types[inner] {
			b.Append([]byte(v))
		}
	}
	return super.NewValue(zctx.LookupTypeSet(inner), super.NormalizeSet(b.Bytes()))
}

func (u *Union) ConsumeAsPartial(val super.Value) {
	if val.IsNull() {
		return
	}
	styp, ok := val.Type().(*super.TypeSet)
	if !ok {
		panic("union: partial not a set type")
	}
	for it := val.Iter(); !it.Done(); {
		typ := styp.Type
		b := it.Next()
		if union, ok := super.TypeUnder(typ).(*super.TypeUnion); ok {
			typ, b = union.Untag(b)
		}
		u.Update(typ, b)
	}
}

func (u *Union) ResultAsPartial(zctx *super.Context) super.Value {
	return u.Result(zctx)
}
