package vcache

import (
	"io"
	"sync"

	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/vector"
	"golang.org/x/sync/errgroup"
)

type nulls struct {
	mu    sync.Mutex
	meta  *csup.Nulls
	local *vector.Bool
	flat  *vector.Bool
}

func (n *nulls) length() uint32 {
	panic("vcacne.nulls.length shouldn't be called")
}

func (n *nulls) fetch(g *errgroup.Group, cctx *csup.Context, reader io.ReaderAt) {
	if n == nil {
		return
	}
	n.mu.Lock()
	if n.meta == nil {
		n.mu.Unlock()
		return
	}
	n.mu.Unlock()
	g.Go(func() error {
		n.mu.Lock()
		defer n.mu.Unlock()
		if n.meta == nil {
			return nil
		}
		n.local = vector.NewBoolEmpty(n.meta.Len(cctx), nil)
		runlens, err := csup.ReadUint32s(n.meta.Runs, reader)
		if err != nil {
			return err
		}
		var null bool
		var off uint32
		b := n.local
		for _, run := range runlens {
			if null {
				for i := range run {
					slot := off + i
					b.Set(slot)
				}
			}
			off += run
			null = !null
		}
		return nil
	})
}

func (n *nulls) flatten(parent *vector.Bool) *vector.Bool {
	if n == nil {
		return parent
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.flat != nil {
		return n.flat
	}
	var flat *vector.Bool
	if parent == nil {
		flat = n.local
	} else if n.local != nil {
		flat = convolve(parent, n.local)
	} else {
		flat = parent
	}
	n.flat = flat
	n.local = nil
	return flat
}

func convolve(parent, child *vector.Bool) *vector.Bool {
	// convolve mixes the parent nulls boolean with a child to compute
	// a new boolean representing the overall sets of nulls by expanding
	// the child to be the same size as the parent and returning that results.
	//XXX this can go faster, but lets make it correct first
	n := parent.Len()
	out := vector.NewBoolEmpty(n, nil)
	var childSlot uint32
	for slot := uint32(0); slot < n; slot++ {
		if parent.Value(slot) {
			out.Set(slot)
		} else {
			if child.Value(childSlot) {
				out.Set(slot)
			}
			childSlot++
		}
	}
	return out
}
