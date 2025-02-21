package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
)

type Any interface {
	Type() super.Type
	Len() uint32
	Serialize(*zcode.Builder, uint32)
}

type Promotable interface {
	Any
	Promote(super.Type) Promotable
}

type Puller interface {
	Pull(done bool) (Any, error)
}

type puller struct {
	vecs []Any
}

func NewPuller(vecs ...Any) Puller {
	return &puller{vecs}
}

func (p *puller) Pull(_ bool) (Any, error) {
	if len(p.vecs) == 0 {
		return nil, nil
	}
	vec := p.vecs[0]
	p.vecs = p.vecs[1:]
	return vec, nil
}
