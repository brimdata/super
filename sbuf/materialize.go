package sbuf

import (
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/vio"
)

type Materializer struct {
	parent vio.Puller
}

var _ Puller = (*Materializer)(nil)

func NewMaterializer(p vio.Puller) Puller {
	return &Materializer{
		parent: p,
	}
}

func (m *Materializer) Pull(done bool) (Batch, error) {
	vec, err := m.parent.Pull(done)
	vec, _ = vector.Unlabel(vec)
	if vec == nil || err != nil {
		return nil, err
	}
	return Materialize(vec), nil
}

func Materialize(vec vector.Any) Batch {
	if vec == nil {
		return nil
	}
	vec, label := vector.Unlabel(vec)
	if vec == nil {
		eoc := EndOfChannel(label)
		return &eoc
	}
	var sb scode.Builder
	vals := make([]super.Value, vec.Len())
	for i := range vec.Len() {
		vals[i] = vector.ValueAt(&sb, vec, i).Copy()
	}
	out := NewArray(vals)
	if label != "" {
		return Label(label, out)
	}
	return out
}

func Dematerialize(sctx *super.Context, batch Batch) vector.Any {
	builder := vector.NewDynamicBuilder()
	for _, val := range batch.Values() {
		builder.Write(val)
	}
	return builder.Build(sctx)
}

type Dematerializer struct {
	sctx   *super.Context
	mu     sync.Mutex
	parent Puller
}

func NewDematerializer(sctx *super.Context, p Puller) *Dematerializer {
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

func CopyVioPuller(w sio.Writer, p vio.Puller) error {
	puller := NewMaterializer(p)
	for {
		b, err := puller.Pull(false)
		if b == nil || err != nil {
			return err
		}
		if err := WriteBatch(w, b); err != nil {
			return err
		}
		b.Unref()
	}
}

func CopyMux(outputs map[string]vio.Pusher, parent vio.Puller) error {
	for {
		vec, err := parent.Pull(false)
		if vec == nil || err != nil {
			return err
		}
		var label string
		vec, label = vector.Unlabel(vec)
		if vec == nil {
			continue
		}
		if w, ok := outputs[label]; ok {
			if err := w.Push(vec); err != nil {
				return err
			}
		}
	}
}

type siopusher struct {
	sio.Writer
}

func NewSioPusher(w sio.Writer) vio.Pusher {
	return &siopusher{w}
}

func (s *siopusher) Push(vec vector.Any) error {
	for i := range vec.Len() {
		var sb scode.Builder
		if err := s.Writer.Write(vector.ValueAt(&sb, vec, i).Copy()); err != nil {
			return err
		}
		sb.Reset()
	}
	return nil
}
