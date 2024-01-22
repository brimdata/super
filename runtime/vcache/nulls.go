package vcache

import (
	"io"
	"sync"

	"github.com/brimdata/zed/vector"
	"github.com/brimdata/zed/vng"
	"golang.org/x/sync/errgroup"
)

type nulls struct {
	mu    sync.Mutex
	meta  *vng.Nulls
	local *vector.Bool
	flat  *vector.Bool
}

func (n *nulls) fetch(g *errgroup.Group, reader io.ReaderAt) {
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
		length := n.meta.Count + n.meta.Values.Len()
		n.local = vector.NewBoolEmpty(length, nil)
		runlens := vng.NewInt64Decoder(n.meta.Runs, reader) //XXX 32-bit reader?
		var null bool
		var off int
		b := n.local
		for {
			run, err := runlens.Next()
			if err != nil {
				if err == io.EOF {
					n.meta = nil
					err = nil
				}
				return err
			}
			if null {
				for i := 0; int64(i) < run; i++ {
					slot := uint32(off + i)
					b.Set(slot)
				}
			}
			off += int(run)
			null = !null
		}
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
