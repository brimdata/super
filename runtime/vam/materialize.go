package vam

import (
	"bytes"
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
	d, _ := vec.(*vector.Dynamic)
	var typ super.Type
	if d == nil {
		typ = vec.Type()
	}
	builder := scode.NewBuilder()
	var vals []super.Value
	n := vec.Len()
	for slot := uint32(0); slot < n; slot++ {
		vec.Serialize(builder, slot)
		if d != nil {
			typ = d.TypeOf(slot)
		}
		val := super.NewValue(typ, bytes.Clone(builder.Bytes().Body()))
		vals = append(vals, val)
		builder.Reset()
	}
	return sbuf.NewArray(vals), nil
}

type dematerializer struct {
	mu     sync.Mutex
	parent sbuf.Puller
}

func NewDematerializer(p sbuf.Puller) vector.Puller {
	return &dematerializer{parent: p}
}

func (d *dematerializer) Pull(done bool) (vector.Any, error) {
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
	return builder.Build(), nil
}
