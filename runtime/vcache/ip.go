package vcache

import (
	"net/netip"
	"sync"

	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
)

type ip struct {
	mu   sync.Mutex
	meta *csup.IP
	vals []netip.Addr
}

func newIP(meta *csup.IP) *ip {
	return &ip{meta: meta}
}

func (i *ip) length() uint32 {
	return i.meta.Count
}

func (*ip) unmarshal(*csup.Context, field.Projection) {}

func (i *ip) project(loader *loader, projection field.Projection) vector.Any {
	if len(projection) > 0 {
		return vector.NewMissing(loader.sctx, i.length())
	}
	return vector.NewIP(i.load(loader))
}

func (i *ip) load(loader *loader) []netip.Addr {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.vals != nil {
		return i.vals
	}
	i.vals = make([]netip.Addr, i.meta.Count)
	table := loadBytesTable(loader, i.meta.Offsets, i.meta.Bytes)
	for k := range table.Len() {
		var ok bool
		if i.vals[k], ok = netip.AddrFromSlice(table.Bytes(k)); !ok {
			panic("malformed ip block")
		}

	}
	return i.vals
}
