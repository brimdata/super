package vam

import (
	"bytes"

	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zcode"
)

type Materializer struct {
	parent vector.Puller
}

var _ zbuf.Puller = (*Materializer)(nil)

func NewMaterializer(p vector.Puller) zbuf.Puller {
	return &Materializer{
		parent: p,
	}
}

func (m *Materializer) Pull(done bool) (zbuf.Batch, error) {
	vec, err := m.parent.Pull(done)
	if vec == nil || err != nil {
		return nil, err
	}
	d, _ := vec.(*vector.Dynamic)
	var typ zed.Type
	if d == nil {
		typ = vec.Type()
	}
	builder := zcode.NewBuilder()
	var vals []zed.Value
	n := vec.Len()
	for slot := uint32(0); slot < n; slot++ {
		vec.Serialize(builder, slot)
		if d != nil {
			typ = d.TypeOf(slot)
		}
		val := zed.NewValue(typ, bytes.Clone(builder.Bytes().Body()))
		vals = append(vals, val)
		builder.Reset()
	}
	return zbuf.NewArray(vals), nil
}
