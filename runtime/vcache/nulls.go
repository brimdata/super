package vcache

import (
	"sync"

	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/vector/bitvec"
)

type nulls struct {
	mu     sync.Mutex
	meta   *csup.Nulls
	flat   bitvec.Bits
	count_ uint32
	parent *nulls
	loaded bool
}

func newNulls(meta *csup.Nulls, parent *nulls) *nulls {
	return &nulls{
		meta:   meta,
		parent: parent,
		count_: meta.Count + parent.count(),
	}
}

func (n *nulls) count() uint32 {
	if n == nil {
		return 0
	}
	return n.count_
}

func (n *nulls) loadWithLock(loader *loader) error {
	local := bitvec.NewFalse(n.meta.Len(loader.cctx))
	runlens, err := csup.ReadUint32s(n.meta.Runs, loader.r)
	if err != nil {
		return err
	}
	var null bool
	var off uint32
	for _, run := range runlens {
		if null {
			for i := range run {
				slot := off + i
				local.Set(slot)
			}
		}
		off += run
		null = !null
	}
	n.flat = flatten(local, n.parent.get(loader))
	n.loaded = true
	return nil
}

func (n *nulls) get(loader *loader) bitvec.Bits {
	if n == nil {
		return bitvec.Zero
	}
	n.mu.Lock()
	if !n.loaded {
		if err := n.loadWithLock(loader); err != nil {
			panic(err)
		}
	}
	n.mu.Unlock()
	return n.flat
}

func flatten(local, parent bitvec.Bits) bitvec.Bits {
	if parent.IsZero() {
		return local
	}
	if local.IsZero() {
		return parent
	}
	return convolve(parent, local)
}

func convolve(parent, child bitvec.Bits) bitvec.Bits {
	// convolve mixes the parent nulls boolean with a child to compute
	// a new boolean representing the overall sets of nulls by expanding
	// the child to be the same size as the parent and returning that results.
	//XXX this can go faster, but lets make it correct first
	n := parent.Len()
	out := bitvec.NewFalse(n)
	var childSlot uint32
	for slot := uint32(0); slot < n; slot++ {
		if parent.IsSet(slot) {
			out.Set(slot)
		} else {
			if child.IsSet(childSlot) {
				out.Set(slot)
			}
			childSlot++
		}
	}
	return out
}
