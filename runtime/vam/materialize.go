package vam

import (
	"sync"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/sio"
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

func (m *Materializer) VectorPuller() vector.Puller {
	return m.parent
}

func (m *Materializer) Pull(done bool) (sbuf.Batch, error) {
	vec, err := m.parent.Pull(done)
	vec, _ = vector.Unlabel(vec)
	if vec == nil || err != nil {
		return nil, err
	}
	return Materialize(vec), nil
}

func Materialize(vec vector.Any) sbuf.Batch {
	if vec == nil {
		return nil
	}
	vec, label := vector.Unlabel(vec)
	if vec == nil {
		eoc := sbuf.EndOfChannel(label)
		return &eoc
	}
	var sb scode.Builder
	vals := make([]super.Value, vec.Len())
	for i := range vec.Len() {
		vals[i] = vector.ValueAt(&sb, vec, i).Copy()
	}
	out := sbuf.NewArray(vals)
	if label != "" {
		return sbuf.Label(label, out)
	}
	return out
}

func Dematerialize(sctx *super.Context, batch sbuf.Batch) vector.Any {
	builder := vector.NewDynamicBuilder()
	for _, val := range batch.Values() {
		builder.Write(val)
	}
	return builder.Build(sctx)
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

func CopyPuller(w sio.Writer, p vector.Puller) error {
	puller := NewMaterializer(p)
	for {
		b, err := puller.Pull(false)
		if b == nil || err != nil {
			return err
		}
		if err := sbuf.WriteBatch(w, b); err != nil {
			return err
		}
		b.Unref()
	}
}

func CopyMux(outputs map[string]vector.Writer, parent vector.Puller) error {
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
			if err := w.Write(vec); err != nil {
				return err
			}
		}
	}
}

type siowriter struct {
	sio.Writer
}

func NewSioWriter(w sio.Writer) vector.Writer {
	return &siowriter{w}
}

func (s *siowriter) Write(vec vector.Any) error {
	for i := range vec.Len() {
		var sb scode.Builder
		if err := s.Writer.Write(vector.ValueAt(&sb, vec, i).Copy()); err != nil {
			return err
		}
		sb.Reset()
	}
	return nil
}
