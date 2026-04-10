package vector

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
)

type NoneTmp struct {
	len uint32
}

func NewNoneTmp(len uint32) *NoneTmp {
	return &NoneTmp{len}
}

func (*NoneTmp) Kind() Kind {
	return KindNone
}

func (n *NoneTmp) Len() uint32 {
	return n.len
}

func (*NoneTmp) Serialize(b *scode.Builder, _ uint32) {
	b.Append(nil)
}

func (*NoneTmp) Type() super.Type {
	return super.TypeNone
}
