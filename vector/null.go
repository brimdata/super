package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type Null struct {
	len uint32
}

func NewNull(len uint32) *Null {
	return &Null{len}
}

func (*Null) Kind() Kind {
	return KindNull
}

func (n *Null) Len() uint32 {
	return n.len
}

func (*Null) Serialize(b *scode.Builder, _ uint32) {
	b.Append(nil)
}

func (*Null) Type() super.Type {
	return super.TypeNull
}
