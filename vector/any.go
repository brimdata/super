package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type Any interface {
	Type() super.Type
	Kind() Kind
	Len() uint32
	Serialize(*scode.Builder, uint32)
}

type Promotable interface {
	Any
	Promote(super.Type) Promotable
}

type Puller interface {
	Pull(done bool) (Any, error)
}

func NewPuller(vecs ...Any) Puller {
	return &puller{vecs}
}

type puller struct {
	vecs []Any
}

func (p *puller) Pull(done bool) (Any, error) {
	if len(p.vecs) == 0 {
		return nil, nil
	}
	vec := p.vecs[0]
	p.vecs = p.vecs[1:]
	return vec, nil
}
