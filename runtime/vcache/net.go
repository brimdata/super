package vcache

import (
	"net/netip"
	"sync"

	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
)

type net struct {
	mu   sync.Mutex
	meta *csup.Net
	vals []netip.Prefix
}

func newNet(meta *csup.Net) *net {
	return &net{meta: meta}
}

func (n *net) length() uint32 {
	return n.meta.Count
}

func (*net) unmarshal(*csup.Context, field.Projection) {}

func (n *net) project(loader *loader, projection field.Projection) vector.Any {
	if len(projection) > 0 {
		return vector.NewMissing(loader.sctx, n.length())
	}
	return vector.NewNet(n.load(loader))
}

func (n *net) load(loader *loader) []netip.Prefix {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.vals != nil {
		return n.vals
	}
	n.vals = make([]netip.Prefix, n.meta.Count)
	table := loadBytesTable(loader, n.meta.Offsets, n.meta.Bytes)
	for k := range table.Len() {
		if err := n.vals[k].UnmarshalBinary(table.Bytes(k)); err != nil {
			panic(err)
		}

	}
	return n.vals
}
