package vcache

import (
	"fmt"
	"net/netip"
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
	"github.com/brimdata/super/zcode"
)

type primitive struct {
	mu   sync.Mutex
	meta *csup.Primitive
	count
	nulls *nulls
	any   any
}

func newPrimitive(cctx *csup.Context, meta *csup.Primitive, nulls *nulls) *primitive {
	return &primitive{
		meta:  meta,
		nulls: nulls,
		count: count{meta.Len(cctx), nulls.count()},
	}
}

func (*primitive) unmarshal(*csup.Context, field.Projection) {}

func (p *primitive) project(loader *loader, projection field.Projection) vector.Any {
	if len(projection) > 0 {
		return vector.NewMissing(loader.sctx, p.length())
	}
	return p.newVector(loader)
}

func (p *primitive) load(loader *loader) any {
	nulls := p.nulls.get(loader)
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.any == nil {
		p.any = p.loadAnyWithLock(loader, nulls)
	}
	return p.any
}

func (p *primitive) loadAnyWithLock(loader *loader, nulls bitvec.Bits) any {
	if p.count.vals == 0 {
		// no vals, just nulls
		return empty(p.meta.Typ, p.length())
	}
	bytes := make([]byte, p.meta.Location.MemLength)
	if err := p.meta.Location.Read(loader.r, bytes); err != nil {
		panic(err)
	}
	length := p.length()
	if !nulls.IsZero() && nulls.Len() != length {
		panic(fmt.Sprintf("BAD NULLS LEN nulls %d %d (cnt.vals %d cnt.null %d) %s", nulls.Len(), length, p.count.vals, p.count.nulls, sup.String(p.meta.Typ)))
	}
	it := zcode.Iter(bytes)
	switch p.meta.Typ.(type) {
	case *super.TypeOfUint8, *super.TypeOfUint16, *super.TypeOfUint32, *super.TypeOfUint64:
		values := make([]uint64, length)
		for slot := uint32(0); slot < length; slot++ {
			if !nulls.IsSet(slot) {
				values[slot] = super.DecodeUint(it.Next())
			}
		}
		return values
	case *super.TypeOfInt8, *super.TypeOfInt16, *super.TypeOfInt32, *super.TypeOfInt64, *super.TypeOfDuration, *super.TypeOfTime:
		values := make([]int64, length)
		for slot := uint32(0); slot < length; slot++ {
			if !nulls.IsSet(slot) {
				values[slot] = super.DecodeInt(it.Next())
			}
		}
		return values
	case *super.TypeOfFloat16, *super.TypeOfFloat32, *super.TypeOfFloat64:
		values := make([]float64, length)
		for slot := uint32(0); slot < length; slot++ {
			if !nulls.IsSet(slot) {
				values[slot] = super.DecodeFloat(it.Next())
			}
		}
		return values
	case *super.TypeOfBool:
		bits := bitvec.NewFalse(length)
		for slot := uint32(0); slot < length; slot++ {
			if !nulls.IsSet(slot) {
				if super.DecodeBool(it.Next()) {
					bits.Set(slot)
				}
			}
		}
		return bits
	case *super.TypeOfBytes:
		bytes := []byte{}
		offs := make([]uint32, length+1)
		var off uint32
		for slot := uint32(0); slot < length; slot++ {
			offs[slot] = off
			if !nulls.IsSet(slot) {
				b := super.DecodeBytes(it.Next())
				bytes = append(bytes, b...)
				off += uint32(len(b))
			}
		}
		offs[length] = off
		return vector.NewBytesTable(offs, bytes)
	case *super.TypeOfString:
		var bytes []byte
		offs := make([]uint32, length+1)
		var off uint32
		for slot := uint32(0); slot < length; slot++ {
			offs[slot] = off
			if !nulls.IsSet(slot) {
				s := super.DecodeString(it.Next())
				bytes = append(bytes, []byte(s)...)
				off += uint32(len(s))
			}
		}
		offs[length] = off
		return vector.NewBytesTable(offs, bytes)
	case *super.TypeOfIP:
		values := make([]netip.Addr, length)
		for slot := uint32(0); slot < length; slot++ {
			if !nulls.IsSet(slot) {
				values[slot] = super.DecodeIP(it.Next())
			}
		}
		return values
	case *super.TypeOfNet:
		values := make([]netip.Prefix, length)
		for slot := uint32(0); slot < length; slot++ {
			if !nulls.IsSet(slot) {
				values[slot] = super.DecodeNet(it.Next())
			}
		}
		return values
	case *super.TypeOfType:
		var bytes []byte
		offs := make([]uint32, length+1)
		var off uint32
		for slot := uint32(0); slot < length; slot++ {
			offs[slot] = off
			if !nulls.IsSet(slot) {
				tv := it.Next()
				bytes = append(bytes, tv...)
				off += uint32(len(tv))
			}
		}
		offs[length] = off
		return vector.NewBytesTable(offs, bytes)
	case *super.TypeEnum:
		values := make([]uint64, length)
		for slot := range length {
			if !nulls.IsSet(slot) {
				values[slot] = super.DecodeUint(it.Next())
			}
		}
		return values
	case *super.TypeOfNull:
		return nil
	}
	panic(fmt.Errorf("internal error: vcache.loadPrimitive got unknown type %#v", p.meta.Typ))
}

func (p *primitive) newVector(loader *loader) vector.Any {
	nulls := p.nulls.get(loader)
	switch typ := p.meta.Typ.(type) {
	case *super.TypeOfUint8, *super.TypeOfUint16, *super.TypeOfUint32, *super.TypeOfUint64:
		return vector.NewUint(typ, p.load(loader).([]uint64), nulls)
	case *super.TypeOfInt8, *super.TypeOfInt16, *super.TypeOfInt32, *super.TypeOfInt64, *super.TypeOfDuration, *super.TypeOfTime:
		return vector.NewInt(typ, p.load(loader).([]int64), nulls)
	case *super.TypeOfFloat16, *super.TypeOfFloat32, *super.TypeOfFloat64:
		return vector.NewFloat(typ, p.load(loader).([]float64), nulls)
	case *super.TypeOfBool:
		return vector.NewBool(p.load(loader).(bitvec.Bits), nulls)
	case *super.TypeOfBytes:
		return vector.NewBytes(p.load(loader).(vector.BytesTable), nulls)
	case *super.TypeOfString:
		return vector.NewString(p.load(loader).(vector.BytesTable), nulls)
	case *super.TypeOfIP:
		return vector.NewIP(p.load(loader).([]netip.Addr), nulls)
	case *super.TypeOfNet:
		return vector.NewNet(p.load(loader).([]netip.Prefix), nulls)
	case *super.TypeOfType:
		return vector.NewTypeValue(p.load(loader).(vector.BytesTable), nulls)
	case *super.TypeEnum:
		return vector.NewEnum(typ, p.load(loader).([]uint64), nulls)
	case *super.TypeOfNull:
		return vector.NewConst(super.Null, p.length(), bitvec.Zero)
	}
	panic(fmt.Errorf("internal error: vcache.loadPrimitive got unknown type %#v", p.meta.Typ))
}

type primitiveLoader struct {
	loader *loader
	shadow *int_
}

var _ vector.PrimitiveLoader = (*primitiveLoader)(nil)

func (i *primitiveLoader) Load() (any, bitvec.Bits) {
	return i.shadow.load(i.loader)
}

func empty(typ super.Type, length uint32) any {
	switch typ := typ.(type) {
	case *super.TypeOfUint8, *super.TypeOfUint16, *super.TypeOfUint32, *super.TypeOfUint64:
		return make([]uint64, length)
	case *super.TypeOfInt8, *super.TypeOfInt16, *super.TypeOfInt32, *super.TypeOfInt64, *super.TypeOfDuration, *super.TypeOfTime:
		return make([]int64, length)
	case *super.TypeOfFloat16, *super.TypeOfFloat32, *super.TypeOfFloat64:
		return make([]float64, length)
	case *super.TypeOfBool:
		return bitvec.NewFalse(length)
	case *super.TypeOfBytes:
		return vector.NewBytesTable(make([]uint32, length+1), nil)
	case *super.TypeOfString:
		return vector.NewBytesTable(make([]uint32, length+1), nil)
	case *super.TypeOfIP:
		return make([]netip.Addr, length)
	case *super.TypeOfNet:
		return make([]netip.Prefix, length)
	case *super.TypeOfType:
		return vector.NewBytesTable(make([]uint32, length+1), nil)
	case *super.TypeOfNull:
		return nil
	default:
		panic(fmt.Sprintf("vcache.empty: unknown type encountered: %T", typ))
	}
}

func extendForNulls[T any](in []T, nulls bitvec.Bits, count count) []T {
	if count.nulls == 0 {
		return in
	}
	out := make([]T, count.length())
	var off int
	for i := range count.length() {
		if !nulls.IsSet(i) {
			out[i] = in[off]
			off++
		}
	}
	return out
}
