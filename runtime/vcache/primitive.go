package vcache

import (
	"fmt"
	"net/netip"
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type primitive struct {
	mu   sync.Mutex
	meta *csup.Primitive
	len  uint32
	any  any
}

func newPrimitive(cctx *csup.Context, meta *csup.Primitive) *primitive {
	return &primitive{meta: meta, len: meta.Len(cctx)}
}

func (p *primitive) length() uint32 {
	return p.len
}

func (*primitive) unmarshal(*csup.Context, field.Projection) {}

func (p *primitive) project(loader *loader, projection field.Projection) vector.Any {
	if len(projection) > 0 {
		return vector.NewMissing(loader.sctx, p.length())
	}
	return p.newVector(loader)
}

func (p *primitive) load(loader *loader) any {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.any == nil {
		p.any = p.loadAnyWithLock(loader)
	}
	return p.any
}

func (p *primitive) loadAnyWithLock(loader *loader) any {
	bytes := make([]byte, p.meta.Location.MemLength)
	if err := p.meta.Location.Read(loader.r, bytes); err != nil {
		panic(err)
	}
	length := p.length()
	it := scode.Iter(bytes)
	switch p.meta.Typ.(type) {
	case *super.TypeOfUint8, *super.TypeOfUint16, *super.TypeOfUint32, *super.TypeOfUint64, *super.TypeEnum:
		values := make([]uint64, 0, length)
		for range length {
			values = append(values, super.DecodeUint(it.Next()))
		}
		return values
	case *super.TypeOfInt8, *super.TypeOfInt16, *super.TypeOfInt32, *super.TypeOfInt64, *super.TypeOfDuration, *super.TypeOfTime:
		values := make([]int64, 0, length)
		for range length {
			values = append(values, super.DecodeInt(it.Next()))
		}
		return values
	case *super.TypeOfFloat16, *super.TypeOfFloat32, *super.TypeOfFloat64:
		values := make([]float64, 0, length)
		for range length {
			values = append(values, super.DecodeFloat(it.Next()))
		}
		return values
	case *super.TypeOfBool:
		bits := bitvec.NewFalse(length)
		for slot := range length {
			if super.DecodeBool(it.Next()) {
				bits.Set(slot)
			}
		}
		return bits
	case *super.TypeOfBytes, *super.TypeOfString, *super.TypeOfType:
		var bytes []byte
		// First offset is always zero.
		offs := make([]uint32, 1, length+1)
		for range length {
			bytes = append(bytes, it.Next()...)
			offs = append(offs, uint32(len(bytes)))
		}
		return vector.NewBytesTable(offs, bytes)
	case *super.TypeOfIP:
		values := make([]netip.Addr, 0, length)
		for range length {
			values = append(values, super.DecodeIP(it.Next()))
		}
		return values
	case *super.TypeOfNet:
		values := make([]netip.Prefix, 0, length)
		for range length {
			values = append(values, super.DecodeNet(it.Next()))
		}
		return values
	case *super.TypeOfNull:
		return nil
	}
	panic(fmt.Errorf("internal error: vcache.loadPrimitive got unknown type %#v", p.meta.Typ))
}

func (p *primitive) newVector(loader *loader) vector.Any {
	switch typ := p.meta.Typ.(type) {
	case *super.TypeOfUint8, *super.TypeOfUint16, *super.TypeOfUint32, *super.TypeOfUint64:
		return vector.NewUint(typ, p.load(loader).([]uint64))
	case *super.TypeOfInt8, *super.TypeOfInt16, *super.TypeOfInt32, *super.TypeOfInt64, *super.TypeOfDuration, *super.TypeOfTime:
		return vector.NewInt(typ, p.load(loader).([]int64))
	case *super.TypeOfFloat16, *super.TypeOfFloat32, *super.TypeOfFloat64:
		return vector.NewFloat(typ, p.load(loader).([]float64))
	case *super.TypeOfBool:
		return vector.NewBool(p.load(loader).(bitvec.Bits))
	case *super.TypeOfBytes:
		return vector.NewBytes(p.load(loader).(vector.BytesTable))
	case *super.TypeOfString:
		return vector.NewString(p.load(loader).(vector.BytesTable))
	case *super.TypeOfIP:
		return vector.NewIP(p.load(loader).([]netip.Addr))
	case *super.TypeOfNet:
		return vector.NewNet(p.load(loader).([]netip.Prefix))
	case *super.TypeOfType:
		return vector.NewTypeValue(p.load(loader).(vector.BytesTable))
	case *super.TypeEnum:
		return vector.NewEnum(typ, p.load(loader).([]uint64))
	case *super.TypeOfNull:
		return vector.NewConst(super.Null, p.length())
	}
	panic(fmt.Errorf("internal error: vcache.loadPrimitive got unknown type %#v", p.meta.Typ))
}
