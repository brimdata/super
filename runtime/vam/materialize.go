package vam

import (
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/vector"
)

type Materializer struct {
	parent vector.Puller
}

var _ sbuf.Puller = (*Materializer)(nil)

func NewMaterializer(p vector.Puller) sbuf.Puller {
	return &Materializer{
		parent: p,
	}
}

func (m *Materializer) Pull(done bool) (sbuf.Batch, error) {
	vec, err := m.parent.Pull(done)
	if vec == nil || err != nil {
		return nil, err
	}
	return Materialize(vec), nil
}

func Materialize(vec vector.Any) sbuf.Batch {
	var sb scode.Builder
	vals := make([]super.Value, vec.Len())
	for i := range vec.Len() {
		vals[i] = vector.ValueAt(&sb, vec, i).Copy()
	}
	return sbuf.NewArray(vals)
}

type Dematerializer struct {
	sctx   *super.Context
	mu     sync.Mutex
	parent sbuf.Puller
}

func NewDematerializer(sctx *super.Context, p sbuf.Puller) *Dematerializer {
	return &Dematerializer{sctx: sctx, parent: p}
}

func (d *Dematerializer) Pull(done bool) (vector.Any, error) {
	return d.ConcurrentPull(done, 0)
}

func (d *Dematerializer) ConcurrentPull(done bool, _ int) (vector.Any, error) {
	d.mu.Lock()
	batch, err := d.parent.Pull(done)
	d.mu.Unlock()
	if batch == nil || err != nil {
		return nil, err
	}
	defer batch.Unref()
	builder := vector.NewDynamicBuilder()
	for _, val := range batch.Values() {
		builder.Write(val)
	}
	return builder.Build(d.sctx), nil
}
