package vcache

import (
	"io"
	"sync"

	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/bitvec"
)

type dynamic struct {
	mu     sync.Mutex
	meta   *csup.Dynamic
	tags   []uint32 // need not be loaded for unordered dynamics
	values []shadow
}

func newDynamic(meta *csup.Dynamic) *dynamic {
	return &dynamic{meta: meta, values: make([]shadow, len(meta.Values))}
}

func (d *dynamic) length() uint32 {
	return d.meta.Length
}

func (d *dynamic) unmarshal(cctx *csup.Context, projection field.Projection) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for k := range d.values {
		if d.values[k] == nil {
			d.values[k] = newShadow(cctx, d.meta.Values[k], nil)
		}
		d.values[k].unmarshal(cctx, projection)
	}
}

func (d *dynamic) project(loader *loader, projection field.Projection) vector.Any {
	vecs := make([]vector.Any, 0, len(d.values))
	for _, shadow := range d.values {
		vecs = append(vecs, shadow.project(loader, projection))
	}
	tags, _ := d.load(loader.r)
	return vector.NewDynamic(tags, vecs)
}

func (d *dynamic) load(r io.ReaderAt) ([]uint32, bitvec.Bits) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.tags != nil {
		return d.tags, bitvec.Zero
	}
	tags, err := csup.ReadUint32s(d.meta.Tags, r)
	if err != nil {
		panic(err)
	}
	d.tags = tags
	return tags, bitvec.Zero
}
