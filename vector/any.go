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

type Puller interface {
	Pull(done bool) (Any, error)
}

type Writer interface {
	Write(Any) error
}

// ValueAt returns the value in vec at slot.  If b is not nil, ValueAt calls b's
// Truncate method and builds the value in it.  To safely reuse b while the
// value is live, call b's Reset method or the value's Copy method.
func ValueAt(b *scode.Builder, vec Any, slot uint32) super.Value {
	var typ super.Type
	if d, ok := vec.(*Dynamic); ok {
		typ = d.TypeOf(slot)
	} else {
		typ = vec.Type()
	}
	if b == nil {
		b = scode.NewBuilder()
	} else {
		b.Truncate()
	}
	vec.Serialize(b, slot)
	return super.NewValue(typ, b.Bytes().Body())
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
